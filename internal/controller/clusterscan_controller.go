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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterscanv1 "github.com/sttvk/audit-http/api/v1"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.github.com,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.github.com,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.github.com,resources=clusterscans/finalizers,verbs=update

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Starting reconcile loop", "ClusterScan", req.NamespacedName)

	var clusterScan clusterscanv1.ClusterScan
	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ClusterScan resource not found. Ignoring since object must be deleted", "ClusterScan", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ClusterScan", "ClusterScan", req.NamespacedName)
		return ctrl.Result{}, err
	}

	var err error
	if clusterScan.Spec.Schedule == "" {
		log.Info("ClusterScan specifies a one-off job", "ClusterScan", req.NamespacedName)
		err = r.reconcileJob(ctx, &clusterScan)
	} else {
		log.Info("ClusterScan specifies a CronJob", "ClusterScan", req.NamespacedName)
		err = r.reconcileCronJob(ctx, &clusterScan)
	}

	if err != nil {
		log.Error(err, "Failed to reconcile Job/CronJob", "ClusterScan", req.NamespacedName)
		clusterScan.Status.LastRunTime = metav1.Now()
		clusterScan.Status.Result = "Failed"
	} else {
		log.Info("Successfully reconciled Job/CronJob", "ClusterScan", req.NamespacedName)
		clusterScan.Status.LastRunTime = metav1.Now()
		clusterScan.Status.Result = "Succeeded"
	}

	if err := r.Status().Update(ctx, &clusterScan); err != nil {
		log.Error(err, "Failed to update ClusterScan status", "ClusterScan", req.NamespacedName)
		return ctrl.Result{}, err
	}

	log.Info("Finished reconcile loop", "ClusterScan", req.NamespacedName)
	return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
}

func (r *ClusterScanReconciler) reconcileJob(ctx context.Context, clusterScan *clusterscanv1.ClusterScan) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling Job", "ClusterScan", clusterScan.Name)

	jobName := fmt.Sprintf("%s-job-%s", clusterScan.Name, string(clusterScan.UID)[:8])
	desired := constructJob(clusterScan, jobName)

	// Check if the Job already exists
	var current batchv1.Job
	err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: clusterScan.Namespace}, &current)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "Failed to get Job", "Job", jobName)
		return err
	}

	if errors.IsNotFound(err) {
		// Job does not exist, create it
		log.Info("Creating Job", "Job", jobName)
		if err := r.Create(ctx, &desired); err != nil {
			log.Error(err, "Failed to create Job", "Job", jobName)
			return err
		}
	} else {
		// Job exists, check if it needs an update
		if !jobEqual(desired.Spec, current.Spec) {
			// Update the Job
			current.Spec = desired.Spec
			if err := r.Update(ctx, &current); err != nil {
				log.Error(err, "Failed to update Job", "Job", jobName)
				return err
			}
			log.Info("Updated Job", "Job", jobName)
		}
	}

	// Wait for the Job to complete
	if err := r.waitForJobCompletion(ctx, &current); err != nil {
		log.Error(err, "Job failed to complete", "Job", jobName)
		return err
	}

	return nil
}

func (r *ClusterScanReconciler) reconcileCronJob(ctx context.Context, clusterScan *clusterscanv1.ClusterScan) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling CronJob", "ClusterScan", clusterScan.Name)

	cronJobName := fmt.Sprintf("%s-cronjob-%s", clusterScan.Name, string(clusterScan.UID)[:8])
	desired := constructCronJob(clusterScan, cronJobName)

	// Check if the CronJob already exists
	var current batchv1.CronJob
	err := r.Get(ctx, types.NamespacedName{Name: cronJobName, Namespace: clusterScan.Namespace}, &current)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "Failed to get CronJob", "CronJob", cronJobName)
		return err
	}

	if errors.IsNotFound(err) {
		// CronJob does not exist, create it
		log.Info("Creating CronJob", "CronJob", cronJobName)
		if err := r.Create(ctx, &desired); err != nil {
			log.Error(err, "Failed to create CronJob", "CronJob", cronJobName)
			return err
		}
	} else {
		// CronJob exists, check if it needs an update
		if !cronJobEqual(desired.Spec, current.Spec) {
			// Update the CronJob
			current.Spec = desired.Spec
			if err := r.Update(ctx, &current); err != nil {
				log.Error(err, "Failed to update CronJob", "CronJob", cronJobName)
				return err
			}
			log.Info("Updated CronJob", "CronJob", cronJobName)
		}
	}

	return nil
}

func (r *ClusterScanReconciler) waitForJobCompletion(ctx context.Context, job *batchv1.Job) error {
	log := log.FromContext(ctx)
	jobName := types.NamespacedName{Name: job.Name, Namespace: job.Namespace}

	for {
		err := r.Get(ctx, jobName, job)
		if err != nil {
			return err
		}

		if job.Status.CompletionTime != nil {
			log.Info("Job completed", "Job", jobName)
			return nil
		}

		log.Info("Waiting for Job to complete", "Job", jobName)
		time.Sleep(5 * time.Second)
	}
}

func jobEqual(desiredSpec, currentSpec batchv1.JobSpec) bool {
	return desiredSpec.Template.Spec.Containers[0].Image == currentSpec.Template.Spec.Containers[0].Image &&
		desiredSpec.Template.Spec.Containers[0].Command[2] == currentSpec.Template.Spec.Containers[0].Command[2]
}

func cronJobEqual(desiredSpec, currentSpec batchv1.CronJobSpec) bool {
	return desiredSpec.Schedule == currentSpec.Schedule &&
		desiredSpec.JobTemplate.Spec.Template.Spec.Containers[0].Image == currentSpec.JobTemplate.Spec.Template.Spec.Containers[0].Image &&
		desiredSpec.JobTemplate.Spec.Template.Spec.Containers[0].Command[2] == currentSpec.JobTemplate.Spec.Template.Spec.Containers[0].Command[2]
}

func constructJob(clusterScan *clusterscanv1.ClusterScan, jobName string) batchv1.Job {
	labels := map[string]string{
		"job-name": jobName,
	}

	commands := buildHTTPCheckCommands(clusterScan)
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: clusterScan.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "cluster-scan",
							Image:   "appropriate/curl",
							Command: []string{"sh", "-c", commands},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}
}

func constructCronJob(clusterScan *clusterscanv1.ClusterScan, cronJobName string) batchv1.CronJob {
	labels := map[string]string{
		"cronjob-name": cronJobName,
	}

	commands := buildHTTPCheckCommands(clusterScan)
	return batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: clusterScan.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: clusterScan.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "cluster-scan",
									Image:   "appropriate/curl",
									Command: []string{"sh", "-c", commands},
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
}

func buildHTTPCheckCommands(clusterScan *clusterscanv1.ClusterScan) string {
	commands := ""
	for _, check := range clusterScan.Spec.HTTPChecks {
		commands += fmt.Sprintf("curl -o /dev/null -s -w \"Pod: %s, Namespace: %s, HTTP Status: %%{http_code}\\n\" %s; ", check.PodName, check.Namespace, check.URL)
	}
	return commands
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterscanv1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}
