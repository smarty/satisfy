# Satisfy Parity Testing

Automated testing suite to verify functional parity between different branches of the satisfy tool.

## Overview

The parity test suite runs comprehensive API tests in isolated Docker containers to ensure that different branches have identical behavior. This is particularly useful for:

- Validating rewrites maintain exact behavior
- Testing refactorings don't change functionality
- Comparing different implementations

## Usage

### Basic Usage

```bash
# Test current branch against master
./parity/test-parity.sh

# Test specific branches
./parity/test-parity.sh master timothy/rewrite

# Compare any two branches
./parity/test-parity.sh feature/old feature/new
```

### Requirements

- Docker installed and running
- Git repository with the branches to test
- Bash shell

## What It Tests

The suite includes 23 comprehensive tests covering:

### CLI Interface
- Version command execution
- Upload command validation
- Download command validation
- Check command validation
- Explicit download subcommand error handling

### Configuration
- Missing required fields
- Invalid compression algorithms
- Negative max-retry values
- Empty dependency lists
- Invalid JSON handling
- Duplicate dependencies

### Data Structures
- Manifest JSON structure
- Local directory expansion (~/)
- Remote address formats (GCS)

### Behavior
- Exit codes (0, 1, 2)
- Flag default values
- Source path priority (source_path → source_directory → source_file)
- Compression algorithms (zstd, gzip, zip)
- Package name validation
- Namespace handling (slashes in package names)

## How It Works

1. **Isolation**: Creates isolated Docker containers for each branch
2. **Git Worktrees**: Uses git worktrees to build both branches independently
3. **Docker Build**: Compiles satisfy binary in Alpine-based containers
4. **Test Execution**: Runs identical test scripts in both containers
5. **Comparison**: Compares exit codes and output (with timestamp normalization)
6. **Reporting**: Shows clear pass/fail status and detailed diffs for failures

## Output

### Success
```
Tests Passed: 23
Tests Failed: 0

All parity tests passed! ✓

The timothy/rewrite branch has exact parity with master
```

### Failure
When parity fails, the script shows:
- Which test failed
- Exit code differences (if any)
- Output differences with context
- Summary of all failures
- Location of detailed diff files

Example:
```
PARITY FAILURES BETWEEN BRANCHES

Baseline: master
Test:     timothy/rewrite

Failed Tests:
  ✗ upload-validation: Exit code mismatch
  ✗ download-empty-deps: Output mismatch

Detailed diffs available in: /tmp/satisfy-parity-XXXXX/results
```

## Test Details

All tests use Docker containers with:
- Alpine Linux base
- Bash shell for test execution
- CA certificates for HTTPS
- Isolated /workspace directory
- No external dependencies required

Tests normalize timestamps in output to avoid false failures due to timing differences.

## Extending the Tests

To add a new test, use the `run_parity_test` function:

```bash
run_parity_test "test-name" \
    "Human-readable test description" \
    "
    # Bash script to run in container
    /satisfy version
    echo \"EXIT:\$?\"
    "
```

The test framework will:
- Execute the script in both containers
- Compare exit codes
- Normalize timestamps
- Diff the outputs
- Report any differences
