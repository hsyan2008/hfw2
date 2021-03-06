package client

import (
	"context"
	"time"

	"github.com/hsyan2008/hfw"
	"github.com/hsyan2008/hfw/common"
	"github.com/hsyan2008/hfw/configs"
	"github.com/hsyan2008/hfw/signal"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	retry = 3
)

//如果有特殊需求，请自行修改
//如GetConn里的authValue，这里是空
//如GetConn里的grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800))
//           和grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(52428800))
func Do(httpCtx *hfw.HTTPContext, c configs.GrpcConfig,
	call func(ctx context.Context, conn *grpc.ClientConn) (interface{}, error),
	timeout time.Duration, opts ...grpc.DialOption,
) (resp interface{}, err error) {

	if httpCtx == nil {
		return nil, common.NewRespErr(500, "nil httpCtx")
	}

	defer func(t time.Time) {
		httpCtx.Infof("Call Grpc:%s CostTime:%s",
			c.ServerName, time.Since(t))
	}(time.Now())

	var retryNum int
	if len(c.Addresses) > 0 {
		retryNum = common.Min(retry, len(c.Addresses)+1)
	} else {
		//服务发现下，len是0
		retryNum = retry
	}

	var conn *grpc.ClientConn
	if c.IsAuth {
		opts = append([]grpc.DialOption{
			grpc.WithUnaryInterceptor(UnaryClientInterceptor),
			grpc.WithStreamInterceptor(StreamClientInterceptor),
		}, opts...)
		conn, err = GetConnWithAuth(signal.GetSignalContext().Ctx, c, "", opts...)
	} else {
		conn, err = GetConnWithDefaultInterceptor(signal.GetSignalContext().Ctx, c, opts...)
	}
	if err != nil {
		return nil, common.NewRespErr(500, err)
	}

	ctx, cancel := context.WithTimeout(httpCtx.Ctx, timeout)
	defer cancel()

	md, ok := metadata.FromOutgoingContext(httpCtx.Ctx)
	if ok == false {
		md = metadata.MD{}
	}

	for i := 0; i < retryNum; i++ {
		select {
		case <-ctx.Done():
			return nil, common.NewRespErr(500, ctx.Err())
		default:
			err = func(httpCtx2 *hfw.HTTPContext) (err error) {
				httpCtx := hfw.NewHTTPContextWithCtx(httpCtx2)
				defer func(t time.Time) {
					httpCtx.Infof("Call Grpc:%s TryTime:%d CostTime:%s",
						c.ServerName, i, time.Since(t))
					httpCtx.Cancel()
				}(time.Now())
				md.Set(common.GrpcTraceIDKey, httpCtx.GetTraceID())
				newCtx := metadata.NewOutgoingContext(ctx, md)
				resp, err = call(newCtx, conn)
				if err == nil {
					return
				}
				httpCtx.Warnf("Call Grpc:%s TryTime:%d Err:%v", c.ServerName, i, err)
				// removeClientConn(c, err)
				return
			}(httpCtx)
			if err == nil || err == context.Canceled || err == context.DeadlineExceeded {
				return
			}
			if _, ok := err.(*common.RespErr); ok {
				return
			}
		}
	}
	if err != nil {
		err = common.NewRespErr(500, err)
	}

	return
}
