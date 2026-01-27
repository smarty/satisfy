#!/usr/bin/env bash
set -eo pipefail

##############################################################################
# satisfy Parity Test Suite
#
# This script tests functional parity between two branches by running
# comprehensive API tests in isolated Docker containers.
#
# Usage:
#   ./parity/test-parity.sh [baseline-branch] [test-branch]
#
# Defaults:
#   baseline-branch: master
#   test-branch: timothy/rewrite
##############################################################################

BASELINE_BRANCH="${1:-master}"
TEST_BRANCH="${2:-timothy/rewrite}"
WORKDIR="/tmp/satisfy-parity-$$"
RESULTS_DIR="$WORKDIR/results"
BASELINE_DIR="$WORKDIR/baseline"
TEST_DIR="$WORKDIR/test"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track overall test status
TESTS_PASSED=0
TESTS_FAILED=0
PARITY_FAILURES=()

##############################################################################
# Helper Functions
##############################################################################

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

log_failure() {
    echo -e "${RED}[FAIL]${NC} $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
}

cleanup() {
    log_info "Cleaning up temporary directory: $WORKDIR"
    rm -rf "$WORKDIR"
}

trap cleanup EXIT

##############################################################################
# Setup Test Environment
##############################################################################

setup_test_environment() {
    log_section "Setting up test environment"

    mkdir -p "$RESULTS_DIR"
    mkdir -p "$BASELINE_DIR"
    mkdir -p "$TEST_DIR"

    log_info "Baseline branch: $BASELINE_BRANCH"
    log_info "Test branch: $TEST_BRANCH"
    log_info "Work directory: $WORKDIR"
}

##############################################################################
# Build Docker Images
##############################################################################

build_docker_image() {
    local branch="$1"
    local output_dir="$2"
    local image_name="satisfy-parity-${branch//\//-}"

    log_info "Building Docker image for branch: $branch" >&2

    # Create a temporary git worktree for this branch
    local worktree="$output_dir/worktree"
    log_info "  Creating git worktree at: $worktree" >&2

    # Use --detach to allow checking out the current branch
    if ! git worktree add --detach "$worktree" "$branch" > "$output_dir/worktree.log" 2>&1; then
        log_failure "Failed to create git worktree for branch: $branch" >&2
        cat "$output_dir/worktree.log" >&2
        exit 1
    fi

    # Verify worktree was created
    if [ ! -d "$worktree" ]; then
        log_failure "Worktree directory not found: $worktree" >&2
        exit 1
    fi

    log_info "  Worktree created successfully" >&2

    # Create a test Dockerfile that has a shell for running tests
    cat > "$output_dir/Dockerfile.test" <<'DOCKERFILE'
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o satisfy ./cmd/satisfy

FROM alpine:latest
RUN apk add --no-cache ca-certificates bash
COPY --from=builder /build/satisfy /satisfy
WORKDIR /workspace
CMD ["/satisfy"]
DOCKERFILE

    # Build Docker image with the test Dockerfile
    log_info "  Building Docker image: $image_name" >&2
    log_info "  Using Dockerfile: $output_dir/Dockerfile.test" >&2
    log_info "  Build context: $worktree" >&2

    if ! docker build -f "$output_dir/Dockerfile.test" -t "$image_name" "$worktree" > "$output_dir/build.log" 2>&1; then
        log_failure "Failed to build Docker image for $branch" >&2
        echo "" >&2
        echo "Build log:" >&2
        cat "$output_dir/build.log" >&2

        # Cleanup worktree before exiting
        git worktree remove "$worktree" --force 2>/dev/null || true
        exit 1
    fi

    log_info "  Docker image built successfully" >&2

    # Cleanup worktree
    log_info "  Cleaning up worktree" >&2
    git worktree remove "$worktree" --force >> "$output_dir/worktree.log" 2>&1 || true

    # Return only the image name to stdout
    echo "$image_name"
}

##############################################################################
# Test Execution Framework
##############################################################################

run_test_in_container() {
    local image="$1"
    local test_name="$2"
    local test_script="$3"
    local output_file="$4"

    # Create a test container with necessary volumes and settings
    docker run --rm \
        --name "satisfy-test-$$-$(date +%s)" \
        -v "$WORKDIR:/workspace" \
        -w /workspace \
        -e "TEST_NAME=$test_name" \
        "$image" \
        /bin/bash -c "$test_script" > "$output_file" 2>&1

    echo $?
}

run_parity_test() {
    local test_name="$1"
    local test_description="$2"
    local test_script="$3"

    log_info "Running test: $test_description"

    local baseline_output="$RESULTS_DIR/baseline-${test_name}.txt"
    local test_output="$RESULTS_DIR/test-${test_name}.txt"
    local baseline_normalized="$RESULTS_DIR/baseline-${test_name}-normalized.txt"
    local test_normalized="$RESULTS_DIR/test-${test_name}-normalized.txt"
    local diff_output="$RESULTS_DIR/diff-${test_name}.txt"

    # Run test on baseline
    local baseline_exit_code
    baseline_exit_code=$(run_test_in_container "$BASELINE_IMAGE" "$test_name" "$test_script" "$baseline_output")

    # Run test on test branch
    local test_exit_code
    test_exit_code=$(run_test_in_container "$TEST_IMAGE" "$test_name" "$test_script" "$test_output")

    # Compare exit codes
    if [ "$baseline_exit_code" != "$test_exit_code" ]; then
        log_failure "$test_description: Exit code mismatch (baseline=$baseline_exit_code, test=$test_exit_code)"
        PARITY_FAILURES+=("$test_name: Exit code mismatch")
        return 1
    fi

    # Normalize outputs by removing timestamps (YYYY/MM/DD HH:MM:SS pattern) and file:line references
    sed -E -e 's/[0-9]{4}\/[0-9]{2}\/[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}/TIMESTAMP/g' \
           -e 's/[a-zA-Z0-9_]+\.go:[0-9]+:/FILE:LINE:/g' \
           "$baseline_output" > "$baseline_normalized"
    sed -E -e 's/[0-9]{4}\/[0-9]{2}\/[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}/TIMESTAMP/g' \
           -e 's/[a-zA-Z0-9_]+\.go:[0-9]+:/FILE:LINE:/g' \
           "$test_output" > "$test_normalized"

    # Compare normalized outputs
    if ! diff -u "$baseline_normalized" "$test_normalized" > "$diff_output" 2>&1; then
        log_failure "$test_description: Output mismatch"
        echo -e "${YELLOW}Diff (timestamps normalized):${NC}"
        head -n 50 "$diff_output" | sed 's/^/    /'

        # Extract and display file:line references from both outputs
        echo ""
        echo -e "${YELLOW}Source locations:${NC}"
        local baseline_locations=$(grep -oE '[a-zA-Z0-9_]+\.go:[0-9]+:' "$baseline_output" | sort -u)
        local test_locations=$(grep -oE '[a-zA-Z0-9_]+\.go:[0-9]+:' "$test_output" | sort -u)
        if [ -n "$baseline_locations" ]; then
            echo -e "  ${BLUE}Baseline:${NC}"
            echo "$baseline_locations" | sed 's/^/    /'
        fi
        if [ -n "$test_locations" ]; then
            echo -e "  ${BLUE}Test:${NC}"
            echo "$test_locations" | sed 's/^/    /'
        fi

        PARITY_FAILURES+=("$test_name: Output mismatch")
        return 1
    fi

    log_success "$test_description"
    return 0
}

##############################################################################
# Test Suite: Version Command
##############################################################################

test_version_command() {
    log_section "Testing: version command"

    # Note: Version will differ between branches, so we just verify format
    run_parity_test "version-command" \
        "version command executes successfully" \
        "/satisfy version 2>&1; echo \"EXIT:\$?\""
}

##############################################################################
# Test Suite: Upload Command - Validation
##############################################################################

test_upload_validation() {
    log_section "Testing: upload command validation"

    # Test 1: Missing required fields
    run_parity_test "upload-missing-fields" \
        "upload with missing required fields" \
        "echo '{}' | /satisfy upload -json=_STDIN_ 2>&1; echo \"EXIT:\$?\""

    # Test 2: Invalid compression algorithm
    run_parity_test "upload-invalid-compression" \
        "upload with invalid compression algorithm" \
        "cat <<'EOF' | /satisfy upload -json=_STDIN_ 2>&1; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"invalid\",
  \"package_name\": \"test-pkg\",
  \"package_version\": \"1.0.0\",
  \"source_directory\": \"/workspace\",
  \"remote_address\": {
    \"scheme\": \"gcs\",
    \"host\": \"test-bucket\"
  }
}
EOF"

    # Test 3: Negative max-retry
    run_parity_test "upload-negative-retry" \
        "upload with negative max-retry" \
        "echo '{}' | /satisfy upload -max-retry=-1 -json=_STDIN_ 2>&1; echo \"EXIT:\$?\""
}

##############################################################################
# Test Suite: Download Command - Validation
##############################################################################

test_download_validation() {
    log_section "Testing: download command validation"

    # Test 1: Empty dependency list
    run_parity_test "download-empty-deps" \
        "download with empty dependency list" \
        "echo '[]' | /satisfy -json=_STDIN_ 2>&1; echo \"EXIT:\$?\""

    # Test 2: Invalid JSON
    run_parity_test "download-invalid-json" \
        "download with invalid JSON" \
        "echo 'not-json' | /satisfy -json=_STDIN_ 2>&1; echo \"EXIT:\$?\""

    # Test 3: Duplicate dependencies
    run_parity_test "download-duplicate-deps" \
        "download with duplicate dependencies" \
        "cat <<'EOF' | /satisfy -json=_STDIN_ 2>&1; echo \"EXIT:\$?\"
[
  {
    \"package_name\": \"test-pkg\",
    \"package_version\": \"1.0.0\",
    \"local_directory\": \"/tmp/test\",
    \"remote_address\": {
      \"scheme\": \"gcs\",
      \"host\": \"test-bucket\"
    }
  },
  {
    \"package_name\": \"test-pkg\",
    \"package_version\": \"1.0.0\",
    \"local_directory\": \"/tmp/test\",
    \"remote_address\": {
      \"scheme\": \"gcs\",
      \"host\": \"test-bucket\"
    }
  }
]
EOF"
}

##############################################################################
# Test Suite: Explicit Download Subcommand Error
##############################################################################

test_download_subcommand_error() {
    log_section "Testing: explicit download subcommand error"

    run_parity_test "download-subcommand-error" \
        "explicit download subcommand causes fatal error" \
        "/satisfy download 2>&1; echo \"EXIT:\$?\""
}

##############################################################################
# Test Suite: Manifest Structure
##############################################################################

test_manifest_structure() {
    log_section "Testing: manifest JSON structure"

    # Create a test source directory with known content
    run_parity_test "manifest-single-file" \
        "manifest structure for single file" \
        "
mkdir -p /workspace/test-source
echo 'test content' > /workspace/test-source/test.txt
cat <<'EOF' | /satisfy upload -json=_STDIN_ -progress=false 2>&1 | grep -E '(Manifest|Archive|Contents|MD5|Size|Compression)' | head -20; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"gzip\",
  \"source_directory\": \"/workspace/test-source\",
  \"package_name\": \"test-manifest\",
  \"package_version\": \"1.0.0\",
  \"remote_address\": {
    \"scheme\": \"file\",
    \"host\": \"/workspace/test-output\"
  }
}
EOF
"
}

##############################################################################
# Test Suite: Local Directory Expansion
##############################################################################

test_local_directory_expansion() {
    log_section "Testing: local directory expansion"

    # Test ~/ expansion
    run_parity_test "local-dir-tilde" \
        "local directory tilde expansion" \
        "
export HOME=/workspace/home
cat <<'EOF' | /satisfy -json=_STDIN_ 2>&1 | grep -i 'expand\|home' | head -10; echo \"EXIT:\$?\"
[
  {
    \"package_name\": \"test-pkg\",
    \"package_version\": \"1.0.0\",
    \"local_directory\": \"~/test\",
    \"remote_address\": {
      \"scheme\": \"gcs\",
      \"host\": \"test-bucket\"
    }
  }
]
EOF
"
}

##############################################################################
# Test Suite: Exit Codes
##############################################################################

test_exit_codes() {
    log_section "Testing: exit codes"

    # Test 1: Success exit code (we can't test actual upload without credentials)
    # So we test the CLI parsing success
    run_parity_test "exit-code-help" \
        "version command returns exit code 0" \
        "/satisfy version >/dev/null 2>&1; echo \"EXIT:\$?\""

    # Test 2: General failure exit code
    run_parity_test "exit-code-failure" \
        "invalid command returns exit code 1" \
        "echo 'invalid' | /satisfy upload -json=_STDIN_ >/dev/null 2>&1; echo \"EXIT:\$?\""
}

##############################################################################
# Test Suite: Flag Defaults
##############################################################################

test_flag_defaults() {
    log_section "Testing: flag default values"

    # These tests verify that the flag parsing works correctly
    # by checking error messages that reference the flag values

    run_parity_test "flag-defaults-upload" \
        "upload flag defaults" \
        "echo '{}' | /satisfy upload -json=_STDIN_ 2>&1 | grep -E 'retry|progress|overwrite' | head -5; echo \"EXIT:\$?\""

    run_parity_test "flag-defaults-download" \
        "download flag defaults" \
        "echo '[]' | /satisfy -json=_STDIN_ 2>&1 | grep -E 'retry|quick|progress' | head -5; echo \"EXIT:\$?\""
}

##############################################################################
# Test Suite: Source Path Priority
##############################################################################

test_source_path_priority() {
    log_section "Testing: source path priority"

    # Create test files in different locations
    run_parity_test "source-path-priority" \
        "source_path takes priority over source_directory and source_file" \
        "
mkdir -p /workspace/path1 /workspace/path2 /workspace/path3
echo 'path1' > /workspace/path1/file.txt
echo 'path2' > /workspace/path2/file.txt
echo 'path3' > /workspace/path3/file.txt

cat <<'EOF' | /satisfy upload -json=_STDIN_ -progress=false 2>&1 | grep -i 'source' | head -10; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"gzip\",
  \"source_path\": \"/workspace/path1\",
  \"source_directory\": \"/workspace/path2\",
  \"source_file\": \"/workspace/path3/file.txt\",
  \"package_name\": \"test-priority\",
  \"package_version\": \"1.0.0\",
  \"remote_address\": {
    \"scheme\": \"file\",
    \"host\": \"/workspace/output\"
  }
}
EOF
"
}

##############################################################################
# Test Suite: Compression Algorithms
##############################################################################

test_compression_algorithms() {
    log_section "Testing: compression algorithms"

    for algo in zstd gzip zip; do
        run_parity_test "compression-$algo" \
            "compression algorithm: $algo" \
            "
mkdir -p /workspace/test-compress-$algo
echo 'test data for $algo' > /workspace/test-compress-$algo/data.txt

cat <<'EOF' | /satisfy upload -json=_STDIN_ -progress=false 2>&1 | grep -i 'compress\|algorithm' | head -10; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"$algo\",
  \"source_directory\": \"/workspace/test-compress-$algo\",
  \"package_name\": \"test-$algo\",
  \"package_version\": \"1.0.0\",
  \"remote_address\": {
    \"scheme\": \"file\",
    \"host\": \"/workspace/output-$algo\"
  }
}
EOF
"
    done
}

##############################################################################
# Test Suite: Package Name Validation
##############################################################################

test_package_name_validation() {
    log_section "Testing: package name validation"

    # Test blank package name
    run_parity_test "package-name-blank" \
        "blank package name rejected" \
        "cat <<'EOF' | /satisfy upload -json=_STDIN_ 2>&1 | grep -i 'package.*name' | head -5; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"gzip\",
  \"package_name\": \"\",
  \"package_version\": \"1.0.0\",
  \"source_directory\": \"/workspace\",
  \"remote_address\": {
    \"scheme\": \"gcs\",
    \"host\": \"test-bucket\"
  }
}
EOF"

    # Test package name with slashes (namespace)
    run_parity_test "package-name-namespace" \
        "package name with namespace (slashes)" \
        "cat <<'EOF' | /satisfy upload -json=_STDIN_ -progress=false 2>&1 | grep -i 'package\|name\|namespace' | head -10; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"gzip\",
  \"package_name\": \"org/project/package\",
  \"package_version\": \"1.0.0\",
  \"source_directory\": \"/workspace\",
  \"remote_address\": {
    \"scheme\": \"file\",
    \"host\": \"/workspace/output\"
  }
}
EOF"
}

##############################################################################
# Test Suite: Remote Address Handling
##############################################################################

test_remote_address() {
    log_section "Testing: remote address handling"

    # Test missing remote address
    run_parity_test "remote-address-missing" \
        "missing remote address rejected" \
        "cat <<'EOF' | /satisfy upload -json=_STDIN_ 2>&1 | grep -i 'remote' | head -5; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"gzip\",
  \"package_name\": \"test\",
  \"package_version\": \"1.0.0\",
  \"source_directory\": \"/workspace\"
}
EOF"

    # Test various remote address schemes
    run_parity_test "remote-address-gcs" \
        "GCS remote address format" \
        "cat <<'EOF' | /satisfy upload -json=_STDIN_ -progress=false 2>&1 | grep -i 'gcs\|scheme\|bucket' | head -10; echo \"EXIT:\$?\"
{
  \"compression_algorithm\": \"gzip\",
  \"package_name\": \"test\",
  \"package_version\": \"1.0.0\",
  \"source_directory\": \"/workspace\",
  \"remote_address\": {
    \"scheme\": \"gcs\",
    \"host\": \"my-bucket\",
    \"path\": \"/prefix/path\"
  }
}
EOF"
}

##############################################################################
# Test Suite: Check Command
##############################################################################

test_check_command() {
    log_section "Testing: check command"

    # Test check command format
    run_parity_test "check-command" \
        "check command validation" \
        "cat <<'EOF' | /satisfy check -json=_STDIN_ 2>&1 | head -20; echo \"EXIT:\$?\"
{
  \"package_name\": \"test-pkg\",
  \"package_version\": \"1.0.0\",
  \"remote_address\": {
    \"scheme\": \"gcs\",
    \"host\": \"test-bucket\"
  }
}
EOF"
}

##############################################################################
# Main Test Execution
##############################################################################

main() {
    log_section "satisfy Parity Test Suite"

    setup_test_environment

    # Build Docker images for both branches
    log_section "Building Docker images"
    BASELINE_IMAGE=$(build_docker_image "$BASELINE_BRANCH" "$BASELINE_DIR")
    TEST_IMAGE=$(build_docker_image "$TEST_BRANCH" "$TEST_DIR")

    log_info "Baseline image: $BASELINE_IMAGE"
    log_info "Test image: $TEST_IMAGE"

    # Run all test suites
    test_version_command
    test_upload_validation
    test_download_validation
    test_download_subcommand_error
    test_manifest_structure
    test_local_directory_expansion
    test_exit_codes
    test_flag_defaults
    test_source_path_priority
    test_compression_algorithms
    test_package_name_validation
    test_remote_address
    test_check_command

    # Report results
    log_section "Test Results"

    echo ""
    echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
    echo ""

    if [ ${#PARITY_FAILURES[@]} -eq 0 ]; then
        log_success "All parity tests passed! ✓"
        echo ""
        echo -e "${GREEN}The $TEST_BRANCH branch has exact parity with $BASELINE_BRANCH${NC}"
        exit 0
    else
        log_failure "Parity failures detected!"
        echo ""
        echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${RED}PARITY FAILURES BETWEEN BRANCHES${NC}"
        echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo ""
        echo -e "Baseline: ${BLUE}$BASELINE_BRANCH${NC}"
        echo -e "Test:     ${BLUE}$TEST_BRANCH${NC}"
        echo ""
        echo -e "${YELLOW}Failed Tests:${NC}"
        for failure in "${PARITY_FAILURES[@]}"; do
            echo -e "  ${RED}✗${NC} $failure"
        done
        echo ""
        echo -e "${YELLOW}Detailed diffs available in:${NC} $RESULTS_DIR"
        echo ""
        exit 1
    fi
}

# Run main function
main
