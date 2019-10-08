package client

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/hsyan2008/hfw/common"
	"github.com/hsyan2008/hfw/configs"
	"github.com/hsyan2008/hfw/grpc/auth"
	"github.com/hsyan2008/hfw/grpc/discovery"
	"github.com/hsyan2008/hfw/grpc/discovery/resolver"
	"github.com/hsyan2008/hfw/grpc/interceptor"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/balancer/roundrobin"
)

type connInstance struct {
	//每个地址的连接实例
	c *grpc.ClientConn
	l *sync.Mutex
}

var connInstanceMap = make(map[string]*connInstance)
var lock = new(sync.Mutex)

func GetConn(ctx context.Context, c configs.GrpcConfig, opt ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	return GetConnWithAuth(ctx, c, "", opt...)
}

func GetConnWithDefaultInterceptor(ctx context.Context, c configs.GrpcConfig, opt ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	opt = append(opt, grpc.WithUnaryInterceptor(interceptor.UnaryClientInterceptor), grpc.WithStreamInterceptor(interceptor.StreamClientInterceptor))
	return GetConnWithAuth(ctx, c, "", opt...)
}

func GetConnWithAuth(ctx context.Context, c configs.GrpcConfig, authValue string,
	opt ...grpc.DialOption) (conn *grpc.ClientConn, err error) {
	if len(c.ServerName) == 0 {
		return nil, errors.New("please specify grpc ServerName")
	}
	//static下，有可能服务名一样而地址不一样，做特殊处理
	if c.ResolverType == resolver.StaticResolver {
		if len(c.Addresses) == 0 {
			return nil, errors.New("please specify grpc Addresses")
		}
		sort.Slice(c.Addresses, func(i, j int) bool { return c.Addresses[i] < c.Addresses[j] })
		c.ServerName = fmt.Sprintf("%s_%s", common.Md5(strings.Join(c.Addresses, "|")), c.ServerName)
	}
	var ok bool
	var p *connInstance
	lock.Lock()
	if p, ok = connInstanceMap[c.ServerName]; !ok {
		p = &connInstance{
			l: new(sync.Mutex),
		}
		connInstanceMap[c.ServerName] = p
		lock.Unlock()
	} else {
		lock.Unlock()
		if p.c != nil {
			return p.c, nil
		}
	}

	p.l.Lock()
	defer p.l.Unlock()

	if p.c != nil {
		return p.c, nil
	}

	scheme, err := discovery.GetAndRegisterResolver(c)
	if err != nil {
		return nil, err
	}
	address := fmt.Sprintf("%s:///%s", scheme, c.ServerName)

	conn, err = newClientConn(ctx, address, c, authValue, opt...)
	if err != nil {
		return
	}

	p.c = conn

	return
}

func newClientConn(ctx context.Context, address string, c configs.GrpcConfig, authValue string, opt ...grpc.DialOption) (*grpc.ClientConn, error) {
	if strings.Contains(address, ":///") {
		// opt = append(opt, grpc.WithBalancerName("round_robin")) //grpc里默认是grpc.WithBalancerName("pick_first")
		if c.BalancerName == "" {
			// 现在默认是round_robin
			c.BalancerName = roundrobin.Name
		}
		opt = append(opt, grpc.WithBalancerName(c.BalancerName))
	}
	if len(c.CertFile) > 0 && !filepath.IsAbs(c.CertFile) {
		c.CertFile = filepath.Join(common.GetAppPath(), c.CertFile)
	}
	if len(c.ServerName) > 0 && len(c.CertFile) > 0 && common.IsExist(c.CertFile) {
		if c.IsAuth {
			opt = append(opt, grpc.WithPerRPCCredentials(auth.NewAuthWithHTTPS(authValue)))
		}
		return NewClientConnWithSecurity(
			ctx,
			address,
			c.CertFile,
			c.ServerName,
			opt...)
	}

	if c.IsAuth {
		opt = append(opt, grpc.WithPerRPCCredentials(auth.NewAuth(authValue)))
	}

	return NewClientConn(ctx, address, opt...)
}
