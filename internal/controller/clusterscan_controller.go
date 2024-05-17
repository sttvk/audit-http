package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterscanv1 "example.com/audit-http/api/v1"
)

// +kubebuilder:rbac:groups=batch.example.com,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch.example.com,resources=clusterscans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var clusterScan clusterscanv1.ClusterScan
	if err := r.Get(ctx, req.NamespacedName, &clusterScan); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ClusterScan resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get ClusterScan")
		return ctrl.Result{}, err
	}

	var err error
	if clusterScan.Spec.Schedule == "" {
		// One-off job
		err = r.createJob(ctx, &clusterScan)
	} else {
		// CronJob for periodic scans
		err = r.createCronJob(ctx, &clusterScan)
	}

	if err != nil {
		clusterScan.Status.LastRunTime = metav1.Now()
		clusterScan.Status.ResultMessages = []string{err.Error()}
		if updateErr := r.Status().Update(ctx, &clusterScan); updateErr != nil {
			log.Error(updateErr, "Failed to update ClusterScan status")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, err
	}

	clusterScan.Status.LastRunTime = metav1.Now()
	clusterScan.Status.ResultMessages = []string{"Scan initiated"}
	if err := r.Status().Update(ctx, &clusterScan); err != nil {
		log.Error(err, "Failed to update ClusterScan status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) createJob(ctx context.Context, clusterScan *clusterscanv1.ClusterScan) error {
	job := constructJob(clusterScan)
	if err := ctrl.SetControllerReference(clusterScan, job, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for Job: %w", err)
	}
	if err := r.Create(ctx, job); err != nil {
		return fmt.Errorf("failed to create Job for ClusterScan: %w", err)
	}
	return nil
}

func (r *ClusterScanReconciler) createCronJob(ctx context.Context, clusterScan *clusterscanv1.ClusterScan) error {
	cronJob := constructCronJob(clusterScan)
	if err := ctrl.SetControllerReference(clusterScan, cronJob, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for CronJob: %w", err)
	}
	if err := r.Create(ctx, cronJob); err != nil {
		return fmt.Errorf("failed to create CronJob for ClusterScan: %w", err)
	}
	return nil
}

func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterscanv1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}

func constructJob(clusterScan *clusterscanv1.ClusterScan) *batchv1.Job {
	commands := buildHTTPCheckCommands(clusterScan)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterScan.Name + "-job",
			Namespace: clusterScan.Namespace,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
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

func constructCronJob(clusterScan *clusterscanv1.ClusterScan) *batchv1.CronJob {
	commands := buildHTTPCheckCommands(clusterScan)
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterScan.Name + "-cronjob",
			Namespace: clusterScan.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: clusterScan.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
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
