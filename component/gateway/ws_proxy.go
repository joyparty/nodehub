package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	"gitlab.haochang.tv/gopkg/nodehub/cluster"
	"gitlab.haochang.tv/gopkg/nodehub/logger"
	"google.golang.org/grpc/metadata"
)

var (
	upgrader = websocket.Upgrader{}
)

// WSProxy 网关服务器
//
// 客户端通过websocket方式连接网关，网关再转发请求到grpc后端服务
type WSProxy struct {
	playground *Playground
	authorizer Authorizer
	server     *http.Server
}

// NewWSProxy 构造函数
func NewWSProxy(
	playground *Playground,
	listenAddr string,
	authorizer Authorizer,
) *WSProxy {
	wp := &WSProxy{
		playground: playground,
		authorizer: authorizer,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/grpc", wp.handle)

	wp.server = &http.Server{
		Addr:    listenAddr,
		Handler: http.HandlerFunc(mux.ServeHTTP),
	}
	return wp
}

// Name 服务名称
func (wp *WSProxy) Name() string {
	return "wsproxy"
}

// CompleteNodeEntry 补全节点信息
func (wp *WSProxy) CompleteNodeEntry(entry *cluster.NodeEntry) {
	entry.Websocket = fmt.Sprintf("ws://%s/grpc", wp.server.Addr)
}

// Start 启动websocket服务器
func (wp *WSProxy) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", wp.server.Addr)
	if err != nil {
		return fmt.Errorf("listen, %w", err)
	}

	go func() {
		if err := wp.server.Serve(l); err != nil && err != http.ErrServerClosed {
			logger.Error("start gateway", "error", err)

			panic(fmt.Errorf("start gateway, %w", err))
		}
	}()

	return nil
}

// Stop 停止websocket服务器
func (wp *WSProxy) Stop(ctx context.Context) error {
	wp.server.Shutdown(ctx)
	wp.playground.Close()
	return nil
}

func (wp *WSProxy) handle(w http.ResponseWriter, r *http.Request) {
	sess, err := wp.newSession(w, r)
	if err != nil {
		logger.Error("initialize session", "error", err, "remoteAddr", r.RemoteAddr)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wp.playground.Handle(ctx, sess)
}

func (wp *WSProxy) newSession(w http.ResponseWriter, r *http.Request) (Session, error) {
	userID, md, ok := wp.authorizer(w, r)
	if !ok {
		return nil, errors.New("deny by authorize")
	} else if userID == "" {
		return nil, errors.New("empty userID")
	} else if md == nil {
		md = metadata.MD{}
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("upgrade websocket, %w", err)
	}

	sess := newWsSession(wsConn)
	sess.SetID(userID)
	sess.SetMetadata(md)

	return sess, nil
}

// Authorizer 身份验证逻辑
//
// 自定义身份验证逻辑，在websocket upgrade之前调用
// 返回的metadata会在此连接的所有grpc request中携带
// 返回的userID如果不为空，则会作为会话唯一标识使用，另外也会被自动加入到metadata中
// 如果返回ok为false，会直接关闭连接
// 因此如果验证不通过之类的错误，需要在这个函数里面自行处理
type Authorizer func(w http.ResponseWriter, r *http.Request) (userID string, md metadata.MD, ok bool)
