package discovery

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hsyan2008/hfw"
	"github.com/hsyan2008/hfw/service/discovery/client"
)

type balancePolicy uint

const (
	UnknownPolicy balancePolicy = iota
	RobinPolicy
	RandPolicy
)

type ConsulResolver struct {
	client      *api.Client
	serviceName string
	tags        []string
	passingOnly bool

	addresses []string
	hasTags   []string

	httpCtx *hfw.HTTPContext

	wg *sync.WaitGroup

	queryOptions *api.QueryOptions

	policy  balancePolicy
	lbIndex uint64
}

var consulResolverMap = make(map[string]*ConsulResolver)
var consulRwLock = new(sync.RWMutex)

func NewConsulResolver(serviceName, address string, opts ...CallOpt) (*ConsulResolver, error) {
	cr := &ConsulResolver{
		serviceName: serviceName,
		passingOnly: true,
	}
	for _, f := range opts {
		f(cr)
	}

	key := fmt.Sprintf("%s_%s_%d_%s", serviceName, address, cr.policy, strings.Join(cr.tags, ","))
	consulRwLock.RLock()
	if cr, ok := consulResolverMap[key]; ok {
		consulRwLock.RUnlock()
		return cr, nil
	}
	consulRwLock.RUnlock()

	consulRwLock.Lock()
	defer consulRwLock.Unlock()

	if cr, ok := consulResolverMap[key]; ok {
		return cr, nil
	}

	httpCtx := hfw.NewHTTPContext()
	client, err := client.NewConsulClient(address)
	if err != nil {
		httpCtx.Fatal("create consul client error", err.Error())
		httpCtx.Cancel()
		return nil, err
	}

	cr.wg = new(sync.WaitGroup)
	cr.httpCtx = httpCtx
	cr.client = client
	cr.queryOptions = (&api.QueryOptions{}).WithContext(httpCtx.Ctx)

	err = cr.resolve()
	if err != nil {
		httpCtx.Cancel()
		return nil, err
	}

	cr.wg.Add(1)
	go cr.watch()

	consulResolverMap[key] = cr

	httpCtx.Info("NewConsulResolver:", key)

	return cr, nil
}
func (consulResolver *ConsulResolver) resolve() (err error) {
	serviceEntries, metaInfo, err := consulResolver.client.Health().ServiceMultipleTags(
		consulResolver.serviceName,
		consulResolver.tags,
		consulResolver.passingOnly,
		consulResolver.queryOptions)
	if err != nil {
		if e, ok := err.(*url.Error); ok {
			if e.Err == context.Canceled {
				return nil
			}
		}
		return
	}

	consulResolver.queryOptions.WaitIndex = metaInfo.LastIndex

	var adds []string
	for _, serviceEntry := range serviceEntries {
		address := fmt.Sprintf("%s:%d", serviceEntry.Service.Address, serviceEntry.Service.Port)
		adds = append(adds, address)
		consulResolver.hasTags = serviceEntry.Service.Tags
	}

	consulResolver.addresses = adds

	return
}

func (consulResolver *ConsulResolver) watch() {
	defer consulResolver.wg.Done()

	for {
		err := consulResolver.resolve()
		if err != nil {
			consulResolver.httpCtx.Fatal("query service entries error:", err.Error())
		}

		select {
		case <-consulResolver.httpCtx.Ctx.Done():
			consulResolver.httpCtx.Cancel()
			return
		default:
		}
	}
}

func (consulResolver *ConsulResolver) Close() {
	consulResolver.httpCtx.Cancel()
	consulResolver.wg.Wait()
}

func (consulResolver *ConsulResolver) Addresses() []string {
	if consulResolver == nil {
		return nil
	}
	return consulResolver.addresses
}

func (consulResolver *ConsulResolver) GetAddress() (address string, err error) {

	if consulResolver == nil {
		return "", errors.New("consul not init")
	}

	addresses := consulResolver.Addresses()
	num := uint64(len(addresses))
	if num == 0 {
		return "", errors.New("addresses is nil")
	}
	if num == 1 {
		return addresses[0], nil
	}
	if consulResolver.policy == RandPolicy {
		//随机
		address = addresses[rand.New(rand.NewSource(time.Now().UnixNano())).Int63n(int64(num))]
	} else {
		//轮询
		address = addresses[consulResolver.lbIndex%num]
		atomic.AddUint64(&consulResolver.lbIndex, 1)
	}
	return
}

func (consulResolver *ConsulResolver) HasTag(tag string) bool {
	for _, v := range consulResolver.hasTags {
		if v == tag {
			return true
		}
	}
	return false
}

type CallOpt func(*ConsulResolver) error

func TagCallOpt(tags ...string) CallOpt {
	return func(cr *ConsulResolver) error {
		cr.tags = tags
		return nil
	}
}

func BalancePolicyCallOpt(balancePolicy balancePolicy) CallOpt {
	return func(cr *ConsulResolver) error {
		cr.policy = balancePolicy
		return nil
	}
}

func PassingOnlyCallOpt(passingOnly bool) CallOpt {
	return func(cr *ConsulResolver) error {
		cr.passingOnly = passingOnly
		return nil
	}
}

var NewTagCallOpt = TagCallOpt
var NewBalancePolicyCallOpt = BalancePolicyCallOpt
