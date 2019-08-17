package client

import (
	"context"
	"time"

	"github.com/hsyan2008/go-logger"
	"github.com/hsyan2008/hfw"
	"github.com/hsyan2008/hfw/common"
	"github.com/hsyan2008/hfw/configs"
	"github.com/hsyan2008/hfw/signal"
	grpc "google.golang.org/grpc"
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
	timeout time.Duration,
) (resp interface{}, err error) {

	if httpCtx == nil {
		httpCtx = &hfw.HTTPContext{Ctx: signal.GetSignalContext().Ctx}
	}
	if httpCtx.Ctx == nil {
		httpCtx.Ctx = signal.GetSignalContext().Ctx
	}

	ctx, cancel := context.WithTimeout(httpCtx.Ctx, timeout)
	defer cancel()

	var conn *grpc.ClientConn

FOR:
	for i := 0; i < common.Min(retry, len(c.Addresses)+1); i++ {
		select {
		case <-ctx.Done():
			break FOR
		default:
			if c.IsAuth {
				conn, err = GetConnWithAuth(signal.GetSignalContext().Ctx, c, "")
			} else {
				conn, err = GetConn(signal.GetSignalContext().Ctx, c)
			}
			if err != nil {
				continue FOR
			}
			func() {
				if logger.Level() == logger.DEBUG {
					httpCtx.Log().Debugf("Call Grpc ServerName: %s start", c.ServerName)
					startTime := time.Now()
					defer func() {
						httpCtx.Log().Debugf("Call Grpc ServerName: %s end CostTime: %s",
							c.ServerName, time.Since(startTime))
					}()
				}
				resp, err = call(ctx, conn)
			}()
			if err == nil {
				return
			}
			httpCtx.Log().Warnf("Call Grpc ServerName: %s %v", c.ServerName, err)
			if err == context.Canceled || err == context.DeadlineExceeded {
				return
			}
			if _, ok := err.(*common.RespErr); ok {
				return
			}
		}
	}

	return
}
