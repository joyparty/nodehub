package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/joyparty/nodehub/cluster"
	"github.com/joyparty/nodehub/event"
	"github.com/joyparty/nodehub/internal/codec"
	"github.com/joyparty/nodehub/logger"
	"github.com/joyparty/nodehub/multicast"
	"github.com/joyparty/nodehub/proto/nh"
	"google.golang.org/grpc/metadata"
)

// Options 网关配置
type Options struct {
	// 网络传输方式
	// 一个网关节点只允许使用一种传输方式
	Transporter Transporter

	// 会话初始化
	// 通过这个配置可以实现自定义的初始化逻辑，例如鉴权
	Initializer Initializer

	// 服务发现注册中心
	Registry *cluster.Registry

	// 集群事件消息总线
	EventBus *event.Bus

	// 对客户端主动下行消息总线
	// 每个网关都会订阅主动消息消息队列
	// 如果消息接收方连接到了当前网关，就会把消息通过对应的会话下发，否则忽略
	Multicast multicast.Subscriber

	// 请求日志记录
	RequestLogger logger.Logger

	// goroutine池，用于处理网关内的各种异步任务
	GoPool GoPool

	// 网络连接保持活跃时间，默认1分钟
	// 如果超过这个时间还没有任何的读写操作，就认为此连接已断开
	KeepaliveInterval time.Duration

	// 每次请求的接口执行超时时间，默认5秒
	RequstTimeout time.Duration

	// 请求消息拦截器
	// 每个请求都会经过这个拦截器，通过之后才会转发到上游服务
	RequestInterceptor RequestInterceptor

	// 连接拦截器
	// 在连接创建之后执行自定义操作，返回错误会中断连接
	ConnectInterceptor ConnectInterceptor

	// 断开连接拦截器
	// 在连接断开前执行自定操作
	DisconnectInterceptor DisconnectInterceptor
}

func newOptions() *Options {
	return &Options{
		KeepaliveInterval:     1 * time.Minute,
		RequstTimeout:         5 * time.Second,
		RequestInterceptor:    defaultRequestInterceptor,
		ConnectInterceptor:    defaultConnectInterceptor,
		DisconnectInterceptor: defaultDisconnectInterceptor,
	}
}

// Validate 有效性检查
func (opt *Options) Validate() error {
	if opt.Registry == nil {
		return errors.New("registry is nil")
	} else if opt.Transporter == nil {
		return errors.New("transporter is nil")
	} else if opt.Initializer == nil {
		return errors.New("initializer is nil")
	} else if opt.EventBus == nil {
		return errors.New("eventBus is nil")
	} else if opt.Multicast == nil {
		return errors.New("multicast subscriber is nil")
	}

	return nil
}

// Option 网关配置选项
type Option func(opt *Options)

// WithRegistry 设置服务注册中心
func WithRegistry(registry *cluster.Registry) Option {
	return func(opt *Options) {
		opt.Registry = registry
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
		opt.Transporter = transporter
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
		opt.Initializer = initializer
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
		opt.RequestInterceptor = interceptor
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
		opt.ConnectInterceptor = interceptor
	}
}

// DisconnectInterceptor 在连接断开前执行自定操作
type DisconnectInterceptor func(ctx context.Context, sess Session)

var defaultDisconnectInterceptor = func(ctx context.Context, sess Session) {}

// WithDisconnectInterceptor 设置断开连接拦截器
func WithDisconnectInterceptor(interceptor DisconnectInterceptor) Option {
	return func(opt *Options) {
		opt.DisconnectInterceptor = interceptor
	}
}

// WithEventBus 设置事件总线
func WithEventBus(bus *event.Bus) Option {
	return func(opt *Options) {
		opt.EventBus = bus
	}
}

// WithMulticast 设置广播组件
func WithMulticast(multicast multicast.Subscriber) Option {
	return func(opt *Options) {
		opt.Multicast = multicast
	}
}

// WithRequestLogger 设置请求日志记录器
func WithRequestLogger(logger logger.Logger) Option {
	return func(opt *Options) {
		opt.RequestLogger = logger
	}
}

// WithKeepaliveInterval 设置网络连接保持活跃时间，默认1分钟
//
// 客户端在没有业务消息的情况下，需要定时向服务器端发送心跳消息
// 服务器端如果检测到客户端连接超过这个时间还没有任何读写，就会认为此连接已断线，会触发主动断线操作
func WithKeepaliveInterval(interval time.Duration) Option {
	return func(opt *Options) {
		opt.KeepaliveInterval = interval.Abs()
	}
}

// WithRequestTimeout 设置每次请求的接口执行超时时间，默认5秒
func WithRequestTimeout(timeout time.Duration) Option {
	return func(opt *Options) {
		opt.RequstTimeout = timeout.Abs()
	}
}

// GoPool goroutine pool
type GoPool interface {
	Submit(task func()) error
}

// WithGoPool 设置goroutine pool
func WithGoPool(pool GoPool) Option {
	return func(opt *Options) {
		opt.GoPool = pool
	}
}

// SetMaxMessageSize 设置客户端消息最大长度，在网关启动之前调用才有效
func SetMaxMessageSize(size int) {
	codec.MaxMessageSize = size
}
