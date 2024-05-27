package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/event"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/multicast"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/grpc/metadata"
)

// Options 网关配置
type Options struct {
	transporter       Transporter
	initializer       Initializer
	registry          *cluster.Registry
	eventBus          *event.Bus
	multicast         multicast.Subscriber
	requestLogger     logger.Logger
	goPool            GoPool
	keepaliveInterval time.Duration

	requestInterceptor    RequestInterceptor
	connectInterceptor    ConnectInterceptor
	disconnectInterceptor DisconnectInterceptor
}

func newOptions() *Options {
	return &Options{
		keepaliveInterval: 1 * time.Minute,

		requestInterceptor:    defaultRequestInterceptor,
		connectInterceptor:    defaultConnectInterceptor,
		disconnectInterceptor: defaultDisconnectInterceptor,
	}
}

// Validate 有效性检查
func (opt *Options) Validate() error {
	if opt.registry == nil {
		return errors.New("registry is nil")
	} else if opt.transporter == nil {
		return errors.New("transporter is nil")
	} else if opt.initializer == nil {
		return errors.New("initializer is nil")
	} else if opt.eventBus == nil {
		return errors.New("eventBus is nil")
	} else if opt.multicast == nil {
		return errors.New("multicast subscriber is nil")
	}

	return nil
}

// Option 网关配置选项
type Option func(opt *Options)

// WithRegistry 设置服务注册中心
func WithRegistry(registry *cluster.Registry) Option {
	return func(opt *Options) {
		opt.registry = registry
	}
}

// Transporter 网关传输层接口
type Transporter interface {
	CompleteNodeEntry(entry *cluster.NodeEntry)
	Serve(ctx context.Context) (chan Session, error)
	Shutdown(ctx context.Context) error
}

// WithTransporter 设置传输层
func WithTransporter(transporter Transporter) Option {
	return func(opt *Options) {
		opt.transporter = transporter
	}
}

// Initializer 连接初始化，会在客户端与网关建立网络连接后调用，初始化完成之后网关才会开始转发请求
//
// metadata用于网关转发的所有grpc request，
// userID会作为session唯一标识使用，会被自动加入到metadata中，
// 如果希望中断初始化且不打印错误日志，return io.EOF错误即可
type Initializer func(ctx context.Context, sess Session) (userID string, md metadata.MD, err error)

// WithInitializer 设置连接初始化逻辑
func WithInitializer(initializer Initializer) Option {
	return func(opt *Options) {
		opt.initializer = initializer
	}
}

// RequestInterceptor 请求拦截器
//
// 每次收到客户端请求都会执行，return pass=false会中断当次请求
type RequestInterceptor func(ctx context.Context, sess Session, req *nh.Request) (pass bool, err error)

var defaultRequestInterceptor = func(ctx context.Context, sess Session, req *nh.Request) (pass bool, err error) {
	return true, nil
}

// WithRequestInterceptor 设置请求拦截器
func WithRequestInterceptor(interceptor RequestInterceptor) Option {
	return func(opt *Options) {
		opt.requestInterceptor = interceptor
	}
}

// ConnectInterceptor 在连接创建之后执行自定义操作，返回错误会中断连接
type ConnectInterceptor func(ctx context.Context, sess Session) error

var defaultConnectInterceptor = func(ctx context.Context, sess Session) error {
	return nil
}

// WithConnectInterceptor 设置连接拦截器
func WithConnectInterceptor(interceptor ConnectInterceptor) Option {
	return func(opt *Options) {
		opt.connectInterceptor = interceptor
	}
}

// DisconnectInterceptor 在连接断开前执行自定操作
type DisconnectInterceptor func(ctx context.Context, sess Session)

var defaultDisconnectInterceptor = func(ctx context.Context, sess Session) {}

// WithDisconnectInterceptor 设置断开连接拦截器
func WithDisconnectInterceptor(interceptor DisconnectInterceptor) Option {
	return func(opt *Options) {
		opt.disconnectInterceptor = interceptor
	}
}

// WithEventBus 设置事件总线
func WithEventBus(bus *event.Bus) Option {
	return func(opt *Options) {
		opt.eventBus = bus
	}
}

// WithMulticast 设置广播组件
func WithMulticast(multicast multicast.Subscriber) Option {
	return func(opt *Options) {
		opt.multicast = multicast
	}
}

// WithRequestLogger 设置请求日志记录器
func WithRequestLogger(logger logger.Logger) Option {
	return func(opt *Options) {
		opt.requestLogger = logger
	}
}

// WithKeepaliveInterval 设置网络连接保持活跃时间，默认1分钟
//
// 客户端在没有业务消息的情况下，需要定时向服务器端发送心跳消息
// 服务器端如果检测到客户端连接超过这个时间还没有任何读写，就会认为此连接已断线，会触发主动断线操作
func WithKeepaliveInterval(interval time.Duration) Option {
	return func(opt *Options) {
		opt.keepaliveInterval = interval.Abs()
	}
}

// GoPool goroutine pool
type GoPool interface {
	Submit(task func()) error
}

// WithGoPool 设置goroutine pool
func WithGoPool(pool GoPool) Option {
	return func(opt *Options) {
		opt.goPool = pool
	}
}
