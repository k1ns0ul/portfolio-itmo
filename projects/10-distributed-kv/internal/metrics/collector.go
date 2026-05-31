package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

var durationBuckets = []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1}

type StateProvider interface {
	KeyCount() int
	RaftState() string
	Term() uint64
	CommitIndex() uint64
	AppliedIndex() uint64
}

type Collector struct {
	provider StateProvider

	mu            sync.Mutex
	requestCounts map[string]uint64
	durationSum   map[string]float64
	durationCount map[string]uint64
	bucketCounts  map[string][]uint64
	totalRequests uint64
}

func NewCollector(provider StateProvider) *Collector {
	return &Collector{
		provider:      provider,
		requestCounts: make(map[string]uint64),
		durationSum:   make(map[string]float64),
		durationCount: make(map[string]uint64),
		bucketCounts:  make(map[string][]uint64),
	}
}

func (c *Collector) Observe(method string, status int, seconds float64) {
	atomic.AddUint64(&c.totalRequests, 1)
	key := method + "|" + strconv.Itoa(status)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.requestCounts[key]++
	c.durationSum[method] += seconds
	c.durationCount[method]++
	if _, ok := c.bucketCounts[method]; !ok {
		c.bucketCounts[method] = make([]uint64, len(durationBuckets))
	}
	for i, bound := range durationBuckets {
		if seconds <= bound {
			c.bucketCounts[method][i]++
		}
	}
}

func (c *Collector) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		c.render(w)
	})
}

func (c *Collector) render(w http.ResponseWriter) {
	fmt.Fprintf(w, "# HELP dkv_keys_total Number of keys in the FSM\n# TYPE dkv_keys_total gauge\n")
	fmt.Fprintf(w, "dkv_keys_total %d\n", c.provider.KeyCount())

	state := c.provider.RaftState()
	fmt.Fprintf(w, "# HELP dkv_raft_state Raft role as a label\n# TYPE dkv_raft_state gauge\n")
	for _, s := range []string{"Leader", "Follower", "Candidate"} {
		value := 0
		if s == state {
			value = 1
		}
		fmt.Fprintf(w, "dkv_raft_state{state=%q} %d\n", s, value)
	}

	fmt.Fprintf(w, "# HELP dkv_raft_term Current raft term\n# TYPE dkv_raft_term gauge\n")
	fmt.Fprintf(w, "dkv_raft_term %d\n", c.provider.Term())
	fmt.Fprintf(w, "# HELP dkv_raft_commit_index Raft commit index\n# TYPE dkv_raft_commit_index gauge\n")
	fmt.Fprintf(w, "dkv_raft_commit_index %d\n", c.provider.CommitIndex())
	fmt.Fprintf(w, "# HELP dkv_raft_applied_index Raft applied index\n# TYPE dkv_raft_applied_index gauge\n")
	fmt.Fprintf(w, "dkv_raft_applied_index %d\n", c.provider.AppliedIndex())

	c.mu.Lock()
	defer c.mu.Unlock()

	fmt.Fprintf(w, "# HELP dkv_requests_total Total requests by method and status\n# TYPE dkv_requests_total counter\n")
	keys := make([]string, 0, len(c.requestCounts))
	for k := range c.requestCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		method, status := splitKey(k)
		fmt.Fprintf(w, "dkv_requests_total{method=%q,status=%q} %d\n", method, status, c.requestCounts[k])
	}

	fmt.Fprintf(w, "# HELP dkv_request_duration_seconds Request duration histogram\n# TYPE dkv_request_duration_seconds histogram\n")
	methods := make([]string, 0, len(c.bucketCounts))
	for m := range c.bucketCounts {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	for _, m := range methods {
		buckets := c.bucketCounts[m]
		for i, bound := range durationBuckets {
			fmt.Fprintf(w, "dkv_request_duration_seconds_bucket{method=%q,le=%q} %d\n",
				m, strconv.FormatFloat(bound, 'g', -1, 64), buckets[i])
		}
		fmt.Fprintf(w, "dkv_request_duration_seconds_bucket{method=%q,le=\"+Inf\"} %d\n", m, c.durationCount[m])
		fmt.Fprintf(w, "dkv_request_duration_seconds_sum{method=%q} %g\n", m, c.durationSum[m])
		fmt.Fprintf(w, "dkv_request_duration_seconds_count{method=%q} %d\n", m, c.durationCount[m])
	}
}

func splitKey(k string) (string, string) {
	for i := 0; i < len(k); i++ {
		if k[i] == '|' {
			return k[:i], k[i+1:]
		}
	}
	return k, ""
}
