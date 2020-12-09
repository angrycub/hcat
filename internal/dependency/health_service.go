package dependency

import (
	"encoding/gob"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcat/dep"
	"github.com/pkg/errors"
)

const (
	HealthAny      = "any"
	HealthPassing  = "passing"
	HealthWarning  = "warning"
	HealthCritical = "critical"
	HealthMaint    = "maintenance"

	NodeMaint    = "_node_maintenance"
	ServiceMaint = "_service_maintenance:"
)

var (
	// Ensure implements
	_ isDependency = (*HealthServiceQuery)(nil)

	// HealthServiceQueryRe is the regular expression to use.
	HealthServiceQueryRe = regexp.MustCompile(`\A` + tagRe + serviceNameRe + dcRe + nearRe + filterRe + `\z`)
)

func init() {
	gob.Register([]*dep.HealthService{})
}

// HealthServiceQuery is the representation of all a service query in Consul.
type HealthServiceQuery struct {
	isConsul
	stopCh chan struct{}

	dc      string
	filters []string
	name    string
	near    string
	tag     string
	connect bool
	opts    QueryOptions
}

// NewHealthServiceQuery processes the strings to build a service dependency.
func NewHealthServiceQuery(s string) (*HealthServiceQuery, error) {
	return healthServiceQuery(s, false)
}

// NewHealthConnect Query processes the strings to build a connect dependency.
func NewHealthConnectQuery(s string) (*HealthServiceQuery, error) {
	return healthServiceQuery(s, true)
}

func healthServiceQuery(s string, connect bool) (*HealthServiceQuery, error) {
	if !HealthServiceQueryRe.MatchString(s) {
		return nil, fmt.Errorf("health.service: invalid format: %q", s)
	}

	m := regexpMatch(HealthServiceQueryRe, s)

	var filters []string
	if filter := m["filter"]; filter != "" {
		split := strings.Split(filter, ",")
		for _, f := range split {
			f = strings.TrimSpace(f)
			switch f {
			case HealthAny,
				HealthPassing,
				HealthWarning,
				HealthCritical,
				HealthMaint:
				filters = append(filters, f)
			case "":
			default:
				return nil, fmt.Errorf(
					"health.service: invalid filter: %q in %q", f, s)
			}
		}
		sort.Strings(filters)
	} else {
		filters = []string{HealthPassing}
	}

	return &HealthServiceQuery{
		stopCh:  make(chan struct{}, 1),
		dc:      m["dc"],
		filters: filters,
		name:    m["name"],
		near:    m["near"],
		tag:     m["tag"],
		connect: connect,
	}, nil
}

// Fetch queries the Consul API defined by the given client and returns a slice
// of HealthService objects.
func (d *HealthServiceQuery) Fetch(clients dep.Clients) (interface{}, *dep.ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts := d.opts.Merge(&QueryOptions{
		Datacenter: d.dc,
		Near:       d.near,
	})

	//log.Printf("[TRACE] %s: GET %s", d, &url.URL{
	//	Path:     "/v1/health/service/%d"+d.name,
	//	RawQuery: opts.String(),
	//})

	// Check if a user-supplied filter was given. If so, we may be querying for
	// more than healthy services, so we need to implement client-side
	// filtering.
	passingOnly := len(d.filters) == 1 && d.filters[0] == HealthPassing

	nodes := clients.Consul().Health().Service
	if d.connect {
		nodes = clients.Consul().Health().Connect
	}
	entries, qm, err := nodes(d.name, d.tag, passingOnly, opts.ToConsulOpts())
	if err != nil {
		return nil, nil, errors.Wrap(err, d.String())
	}

	//log.Printf("[TRACE] %s: returned %d results", d, len(entries))

	list := make([]*dep.HealthService, 0, len(entries))
	for _, entry := range entries {
		// Get the status of this service from its checks.
		status := entry.Checks.AggregatedStatus()

		// If we are not checking only healthy services, filter out services
		// that do not match the given filter.
		if !acceptStatus(d.filters, status) {
			continue
		}

		// Get the address of the service, falling back to the address of the
		// node.
		address := entry.Service.Address
		if address == "" {
			address = entry.Node.Address
		}

		list = append(list, &dep.HealthService{
			Node:                entry.Node.Node,
			NodeID:              entry.Node.ID,
			NodeAddress:         entry.Node.Address,
			NodeDatacenter:      entry.Node.Datacenter,
			NodeTaggedAddresses: entry.Node.TaggedAddresses,
			NodeMeta:            entry.Node.Meta,
			ServiceMeta:         entry.Service.Meta,
			Address:             address,
			ID:                  entry.Service.ID,
			Name:                entry.Service.Service,
			Tags: dep.ServiceTags(
				deepCopyAndSortTags(entry.Service.Tags)),
			Status:    status,
			Checks:    entry.Checks,
			Port:      entry.Service.Port,
			Weights:   entry.Service.Weights,
			Namespace: entry.Service.Namespace,
		})
	}

	//log.Printf("[TRACE] %s: returned %d results after filtering", d, len(list))

	// Sort unless the user explicitly asked for nearness
	if d.near == "" {
		sort.Stable(ByNodeThenID(list))
	}

	rm := &dep.ResponseMetadata{
		LastIndex:   qm.LastIndex,
		LastContact: qm.LastContact,
	}

	return list, rm, nil
}

// CanShare returns a boolean if this dependency is shareable.
func (d *HealthServiceQuery) CanShare() bool {
	return true
}

// Stop halts the dependency's fetch function.
func (d *HealthServiceQuery) Stop() {
	close(d.stopCh)
}

// String returns the human-friendly version of this dependency.
func (d *HealthServiceQuery) String() string {
	name := d.name
	if d.tag != "" {
		name = d.tag + "." + name
	}
	if d.dc != "" {
		name = name + "@" + d.dc
	}
	if d.near != "" {
		name = name + "~" + d.near
	}
	if len(d.filters) > 0 {
		name = name + "|" + strings.Join(d.filters, ",")
	}
	return fmt.Sprintf("health.service(%s)", name)
}

func (d *HealthServiceQuery) SetOptions(opts QueryOptions) {
	d.opts = opts
}

// acceptStatus allows us to check if a slice of health checks pass this filter.
func acceptStatus(list []string, s string) bool {
	for _, status := range list {
		if status == s || status == HealthAny {
			return true
		}
	}
	return false
}

// ByNodeThenID is a sortable slice of Service
type ByNodeThenID []*dep.HealthService

// Len, Swap, and Less are used to implement the sort.Sort interface.
func (s ByNodeThenID) Len() int      { return len(s) }
func (s ByNodeThenID) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByNodeThenID) Less(i, j int) bool {
	if s[i].Node < s[j].Node {
		return true
	} else if s[i].Node == s[j].Node {
		return s[i].ID <= s[j].ID
	}
	return false
}
