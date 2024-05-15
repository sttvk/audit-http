package controller

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterscanv1 "example.com/audit-http/api/v1"
)

// +kubebuilder:rbac:groups=example.com,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=example.com,resources=clusterscans/status,verbs=get;update;patch
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
		log.Error(err, "unable to fetch ClusterScan")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if clusterScan.Spec.Schedule == "" {
		// One-off job
		job := constructJob(&clusterScan)
		if err := r.Create(ctx, job); err != nil {
			log.Error(err, "Failed to create Job for ClusterScan", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			return ctrl.Result{}, err
		}
	} else {
		// CronJob for periodic scans
		cronJob := constructCronJob(&clusterScan)
		if err := r.Create(ctx, cronJob); err != nil {
			log.Error(err, "Failed to create CronJob for ClusterScan", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)
			return ctrl.Result{}, err
		}
	}

	clusterScan.Status.LastRunTime = metav1.Now()
	clusterScan.Status.ResultMessage = "Scan initiated"
	if err := r.Status().Update(ctx, &clusterScan); err != nil {
		log.Error(err, "failed to update ClusterScan status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clusterscanv1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}).
		Complete(r)
}

func constructJob(clusterScan *clusterscanv1.ClusterScan) *batchv1.Job {
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
							Command: []string{"sh", "-c", "kubectl get pods --all-namespaces"},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}
}

func constructCronJob(clusterScan *clusterscanv1.ClusterScan) *batchv1.CronJob {
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
									Command: []string{"sh", "-c", "kubectl get pods --all-namespaces"},
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
