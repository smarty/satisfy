package satisfy

import (
	"context"
	"crypto/md5"
	"errors"
	"sync"

	"github.com/smarty/satisfy/v2/internal/fsys"
	"github.com/smarty/satisfy/v2/internal/model"
	"github.com/smarty/satisfy/v2/internal/packaging"
)

// Install makes each dependency's local directory hold the requested package
// version, downloading only what is missing or out of date. Dependencies are
// installed concurrently; the returned error joins the failure of every
// dependency that could not be installed.
func (this *Client) Install(ctx context.Context, dependencies ...Dependency) error {
	if err := ValidateDependencies(dependencies...); err != nil {
		return err
	}

	disk := fsys.NewDiskFileSystem("")
	installer := packaging.NewPackageInstaller(this.storage, disk, this.logger, this.progress)
	integrity := packaging.NewCompoundIntegrityCheck(
		packaging.NewFileListingIntegrityChecker(disk),
		packaging.NewFileContentIntegrityCheck(md5.New, disk, this.verifyContents),
	)
	errs := runConcurrently(ctx, this.concurrency, dependencies, func(dependency model.Dependency) error {
		return packaging.NewDependencyResolver(disk, integrity, installer, dependency, this.logger).Resolve(ctx)
	})
	return errors.Join(errs...)
}

// runConcurrently calls work for each item, with at most limit calls in flight
// at once, and returns the errors encountered. Once ctx is done, no further
// work starts and ctx's error is among those returned.
func runConcurrently[T any](ctx context.Context, limit int, items []T, work func(T) error) (errs []error) {
	var (
		waiter    sync.WaitGroup
		lock      sync.Mutex
		semaphore = make(chan struct{}, max(limit, 1))
	)
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		select {
		case semaphore <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		waiter.Go(func() {
			defer func() { <-semaphore }()
			if err := work(item); err != nil {
				lock.Lock()
				errs = append(errs, err)
				lock.Unlock()
			}
		})
	}
	waiter.Wait()
	if err := ctx.Err(); err != nil {
		errs = append(errs, err)
	}
	return errs
}
