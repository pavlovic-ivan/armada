package repository

import (
	"fmt"
	"github.com/G-Research/k8s-batch/internal/armada/api"
	"github.com/go-redis/redis"
	"github.com/gogo/protobuf/proto"
	"github.com/kjk/betterguid"
	"time"
)

const jobObjectPrefix = "Job:"
const jobQueuePrefix = "Job:Queue:"
const jobLeasedPrefix = "Job:Leased:"

type JobRepository interface {
	AddJob(request *api.JobRequest) (string, error)
	PeekQueue(queue string, limit int64) ([]*api.Job, error)
	FilterActiveQueues(queueNames []string) ([]string, error)
	TryLeaseJobs(queue string, jobs []*api.Job) ([]*api.Job, error)
}

type RedisJobRepository struct {
	Db *redis.Client
}

func (repo RedisJobRepository) AddJob(request *api.JobRequest) (string, error) {

	pipe := repo.Db.TxPipeline()

	job := createJob(request)
	jobData, e := proto.Marshal(job)
	if e != nil {
		return "", e
	}

	pipe.ZAdd(jobQueuePrefix+job.Queue, redis.Z{
		Member: job.Id,
		Score:  job.Priority})

	pipe.SAdd(jobObjectPrefix+job.Id, jobData)

	fmt.Println(job)

	_, e = pipe.Exec()
	return job.Id, e
}

func (repo RedisJobRepository) PeekQueue(queue string, limit int64) ([]*api.Job, error) {
	ids, e := repo.Db.ZRange(jobQueuePrefix+queue, 0, limit-1).Result()
	if e != nil {
		return nil, e
	}
	return repo.GetJobsByIds(ids)
}

// returns list of jobs which are successfully leased
func (repo RedisJobRepository) TryLeaseJobs(queue string, jobs []*api.Job) ([]*api.Job, error) {
	now := float64(time.Now().Unix())
	pipe := repo.Db.Pipeline()
	cmds := make(map[*api.Job]*redis.Cmd)
	for _, job := range jobs{
		cmds[job] = zmove(pipe, jobQueuePrefix+queue, jobLeasedPrefix+queue, job.Id, now)
	}

	_, e := pipe.Exec()
	if e != nil {
		return nil, e
	}

	leasedJobs := make([]*api.Job, 0)
	for job, cmd := range cmds {
		modified, e := cmd.Int()
		if e != nil && modified > 0 {
			leasedJobs = append(leasedJobs, job)
		}
	}
	return leasedJobs, nil
}


func (repo RedisJobRepository) GetJobsByIds(ids []string) ([]*api.Job, error) {
	pipe := repo.Db.Pipeline()
	var cmds []*redis.StringCmd
	for _, id := range ids {
		cmds = append(cmds, pipe.Get(id))
	}
	_, e := pipe.Exec()
	if e != nil {
		return nil, e
	}

	var jobs []*api.Job
	for _, cmd := range cmds {
		d, _ := cmd.Bytes()
		job := &api.Job{}
		e = proto.Unmarshal(d, job)
		if e != nil {
			return nil, e
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (repo RedisJobRepository) FilterActiveQueues(queueNames []string) ([]string, error) {
	pipe := repo.Db.Pipeline()
	cmds := make(map[string]*redis.IntCmd)
	for _, queue := range queueNames {
		// empty (even sorted) sets gets deleted by redis automatically
		cmds[queue] = pipe.Exists(jobQueuePrefix+queue)
	}
	_, e := pipe.Exec()
	if e != nil {
		return nil, e
	}

	var active []string
	for queue, cmd := range cmds {
		if cmd.Val() > 0 {
			active = append(active, queue)
		}
	}
	return active, nil
}

func createJob(jobRequest *api.JobRequest) *api.Job {
	j := api.Job{
		Id:       betterguid.New(),
		Queue:    jobRequest.Queue,
		JobSetId: jobRequest.JobSetId,

		Priority: jobRequest.Priority,

		PodSpec: jobRequest.PodSpec,
		Created: time.Now(),
	}
	return &j
}

const zmoveScript = `
local exists = redis.call('ZREM', KEYS[1], ARGV[1])
if exists then
	return redis.call('ZADD', KEYS[2], ARGV[2], ARGV[1])
else
	return 0
end
`
func zmove(db redis.Cmdable, from string , to string, key string, score float64) *redis.Cmd {
	return db.Eval(zmoveScript, []string {from, to}, key, score)
}
