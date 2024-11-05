/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"time"

	kbatch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	batchv1 "github.com/abdurrehman107/backup-scheduler/api/v1"
)

// CronJobReconciler reconciles a CronJob object
type CronJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var (
    scheduledTimeAnnotation = "batch.tutorial.kubebuilder.io/scheduled-at"
)


// +kubebuilder:rbac:groups=batch.backupscheduler.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.backupscheduler.io,resources=cronjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch.backupscheduler.io,resources=cronjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch.backupscheduler.io,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.backupscheduler.io,resources=jobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var my_job batchv1.CronJob
	if err := r.Get(ctx, req.NamespacedName, &my_job); err != nil {
		log.Error(err, "unable to fetch cronJob")
	}
	// List all jobs 
	var childJobs kbatch.JobList
    if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
        log.Error(err, "unable to list child Jobs")
        return ctrl.Result{}, err
    }
	var activeJobs []*kbatch.Job
	var successfulJobs []*kbatch.Job
	var failedJobs []*kbatch.Job
	var mostRecentTime *time.Time
	// Check if job is finished or not 
	isJobFinished := func(job *kbatch.Job) (bool, kbatch.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if c.Type == kbatch.JobComplete { // the job has completed 
				return true, c.Type
			}
		}
		return false, ""
	}
	getScheduledTimeForJob := func (job *kbatch.Job) (*time.Time, error) {
		timeRaw := job.Annotations[scheduledTimeAnnotation]
		if len(timeRaw) == 0 {
			return nil, nil
		}
		timeParsed, err := time.Parse(time.RFC1123, timeRaw)
		if err != nil {
			return nil, err 
		}
		return &timeParsed, nil
	}
	for _, job := range childJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "":
			activeJobs = append(activeJobs, &job)
		case kbatch.JobFailed:
			failedJobs = append(failedJobs, &job)
		case kbatch.JobComplete:
			successfulJobs = append(successfulJobs, &job)
		}
		scheudledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			log.Error(err, "unable to get scheudled time for job")
			continue
		}
		if scheudledTimeForJob != nil {
			if mostRecentTime == nil || mostRecentTime.Before(*scheudledTimeForJob) {
				mostRecentTime = scheudledTimeForJob
			}
		}
	}
	if mostRecentTime != nil {
		my_job.Status.LastScheduledTime := &metav1.Time{Time: &mostRecentTime}
	} else {
		my_job.Status.LastScheduledTime := nil
	}



	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Named("cronjob").
		Complete(r)
}
