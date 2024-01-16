package install

import (
	"context"
	"errors"
	"fmt"
	"github.com/glasskube/glasskube/api/v1alpha1"
	"github.com/glasskube/glasskube/api/v1alpha1/condition"
	"github.com/glasskube/glasskube/pkg/client"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

// Install creates a new v1alpha1.Package custom resource in the cluster, and blocks until this resource has either
// status Ready or Failed.
func Install(pkgClient *client.PackageV1Alpha1Client, ctx context.Context, packageName string) (*client.PackageStatus, error) {
	pkg := client.NewPackage(packageName)
	err := pkgClient.Packages().Create(ctx, pkg)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Installing %v.\n", packageName)

	status, err := awaitInstall(pkgClient, ctx, pkg.GetUID())
	if err != nil {
		return nil, err
	}

	return status, nil
}

func awaitInstall(pkgClient *client.PackageV1Alpha1Client, ctx context.Context, pkgUID types.UID) (*client.PackageStatus, error) {
	watcher, err := pkgClient.Packages().Watch(ctx)
	if err != nil {
		return nil, err
	}

	defer watcher.Stop()
	for event := range watcher.ResultChan() {
		if obj, ok := event.Object.(*v1alpha1.Package); ok && obj.GetUID() == pkgUID {
			if event.Type == watch.Added || event.Type == watch.Modified {
				if status := getStatus(&obj.Status); status != nil {
					return status, nil
				}
			} else if event.Type == watch.Deleted {
				return nil, errors.New("created package has been deleted unexpectedly")
			}
		}
	}
	return nil, errors.New("failed to confirm package installation status")
}

func getStatus(status *v1alpha1.PackageStatus) *client.PackageStatus {
	readyCnd := meta.FindStatusCondition((*status).Conditions, condition.Ready)
	if readyCnd != nil && readyCnd.Status == v1.ConditionTrue {
		return newPackageStatus(readyCnd)
	}
	failedCnd := meta.FindStatusCondition((*status).Conditions, condition.Failed)
	if failedCnd != nil && failedCnd.Status == v1.ConditionTrue {
		return newPackageStatus(failedCnd)
	}
	return nil
}

func newPackageStatus(cnd *v1.Condition) *client.PackageStatus {
	return &client.PackageStatus{
		Status:  cnd.Type,
		Reason:  cnd.Reason,
		Message: cnd.Message,
	}
}