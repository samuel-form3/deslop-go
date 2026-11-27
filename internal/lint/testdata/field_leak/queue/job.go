package queue

// Job is plain API data, after a Kubernetes custom resource: callers are
// meant to work on its fields. Its only methods are generated, so it guards
// nothing, and nothing leaks.
type Job struct {
	Kind   string
	Spec   JobSpec
	Status JobStatus
}

type JobSpec struct {
	Tasks []string
}

type JobStatus struct {
	Done []string
}
