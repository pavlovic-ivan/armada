package service

import (
	"context"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	listers "k8s.io/client-go/listers/core/v1"

	"github.com/G-Research/k8s-batch/internal/armada/api"
	"github.com/G-Research/k8s-batch/internal/common"
	commonUtil "github.com/G-Research/k8s-batch/internal/common/util"
	"github.com/G-Research/k8s-batch/internal/executor/util"
)

type JobLeaseService struct {
	PodLister         listers.PodLister
	QueueClient       api.AggregatedQueueClient
	CleanupService    PodCleanupService
	ClusterId         string
	SubmittedJobCache util.PodCache
}

func (jobLeaseService JobLeaseService) RequestJobLeases(availableResource *common.ComputeResources) ([]*api.Job, error) {
	leaseRequest := api.LeaseRequest{
		ClusterId: jobLeaseService.ClusterId,
		Resources: *availableResource,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := jobLeaseService.QueueClient.LeaseJobs(ctx, &leaseRequest)

	if err != nil {
		return make([]*api.Job, 0), err
	}

	return response.Job, nil
}

func (jobLeaseService JobLeaseService) ManageJobLeases() {
	allManagedPods, err := jobLeaseService.PodLister.List(util.GetManagedPodSelector())
	if err != nil {
		log.Errorf("Failed to manage job leases due to %s", err)
		return
	}

	podsToRenew := allManagedPods
	podsToRenew = util.MergePodList(podsToRenew, jobLeaseService.SubmittedJobCache.GetAll())

	podsToCleanup := getFinishedPods(allManagedPods)

	jobLeaseService.renewJobLeases(podsToRenew)

	err = jobLeaseService.ReportDone(podsToCleanup)
	if err != nil {
		log.Errorf("Failed reporting jobs as done because %s", err)
	}

	jobLeaseService.CleanupService.DeletePods(podsToCleanup)
}

func (jobLeaseService JobLeaseService) ReturnLease(pod *v1.Pod) error {
	jobId := util.ExtractJobId(pod)
	ctx, cancel := common.ContextWithDefaultTimeout()
	defer cancel()
	log.Infof("Returning lease for job %s", jobId)
	_, err := jobLeaseService.QueueClient.ReturnLease(ctx, &api.ReturnLeaseRequest{ClusterId: jobLeaseService.ClusterId, JobId: jobId})

	return err
}

func (jobLeaseService JobLeaseService) ReportDone(pods []*v1.Pod) error {
	if len(pods) <= 0 {
		return nil
	}
	jobIds := util.ExtractJobIds(pods)

	ctx, cancel := common.ContextWithDefaultTimeout()
	defer cancel()
	log.Infof("Reporting done for jobs %s", strings.Join(jobIds, ","))
	_, err := jobLeaseService.QueueClient.ReportDone(ctx, &api.IdList{Ids: jobIds})

	return err
}

func (jobLeaseService JobLeaseService) renewJobLeases(pods []*v1.Pod) {
	if len(pods) <= 0 {
		return
	}
	jobIds := util.ExtractJobIds(pods)
	log.Infof("Renewing lease for %s", strings.Join(jobIds, ","))

	ctx, cancel := common.ContextWithDefaultTimeout()
	defer cancel()
	renewedJobIds, err := jobLeaseService.QueueClient.RenewLease(ctx, &api.RenewLeaseRequest{ClusterId: jobLeaseService.ClusterId, Ids: jobIds})
	if err != nil {
		log.Errorf("Failed to renew lease for jobs because %s", err)
		return
	}

	failedIds := commonUtil.SubtractStringList(jobIds, renewedJobIds.Ids)
	failedPods := filterPodsByJobId(pods, failedIds)
	if len(failedIds) > 0 {
		log.Errorf("Failed to renew job lease for jobs %s", strings.Join(failedIds, ","))
		jobLeaseService.CleanupService.DeletePods(failedPods)
	}
}

func getRunningPods(pods []*v1.Pod) []*v1.Pod {
	runningPods := make([]*v1.Pod, 0)

	for _, pod := range pods {
		if !IsPodReadyForCleanup(pod) {
			runningPods = append(runningPods, pod)
		}
	}

	return runningPods
}

func getFinishedPods(pods []*v1.Pod) []*v1.Pod {
	finishedPods := make([]*v1.Pod, 0)

	for _, pod := range pods {
		if IsPodReadyForCleanup(pod) {
			finishedPods = append(finishedPods, pod)
		}
	}

	return finishedPods
}

func filterPodsByJobId(pods []*v1.Pod, ids []string) []*v1.Pod {
	reportedIdSet := commonUtil.StringListToSet(ids)
	filteredPods := make([]*v1.Pod, 0)
	for _, pod := range pods {
		if reportedIdSet[util.ExtractJobId(pod)] {
			filteredPods = append(filteredPods, pod)
		}
	}
	return filteredPods
}
