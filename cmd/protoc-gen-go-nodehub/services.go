package main

import (
	"fmt"

	"github.com/joyparty/gokit"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Method struct {
	*protogen.Method
	ReplyCode protoreflect.Value
}

type Service struct {
	*protogen.Service
	Code    protoreflect.Value
	Methods []Method
}

func genMethodReplyCodes(file *protogen.File, g *protogen.GeneratedFile) bool {
	services := lo.Map(file.Services, func(s *protogen.Service, _ int) Service {
		serviceCode, ok := getServiceCode(s)
		if !ok {
			return Service{}
		}

		methods := lo.Map(s.Methods, func(m *protogen.Method, _ int) Method {
			replyCode, ok := getReplyCode(m)
			if ok {
				return Method{Method: m, ReplyCode: replyCode}
			}
			return Method{}
		})
		methods = lo.Filter(methods, func(m Method, _ int) bool { return m.ReplyCode.IsValid() })

		return Service{
			Service: s,
			Code:    serviceCode,
			Methods: methods,
		}
	})
	services = lo.Filter(services, func(s Service, _ int) bool {
		return s.Code.IsValid()
	})

	lo.ForEach(services, func(s Service, _ int) {
		g.P()
		g.P("// ", s.GoName, "_MethodReplyCodes 每个grpc方法返回值对应的nodehub.Reply.code")
		g.P("var ", s.GoName, "_MethodReplyCodes = map[string]int32{")

		for _, m := range s.Methods {
			methodName := fmt.Sprintf("/%s/%s", s.Desc.FullName(), m.GoName)
			g.P(fmt.Sprintf("\t%q: %v,", methodName, m.ReplyCode.Interface()))
		}

		g.P("}")
	})

	return len(services) > 0
}

func getServiceCode(s *protogen.Service) (val protoreflect.Value, ok bool) {
	options := s.Desc.Options().(*descriptorpb.ServiceOptions)
	if options == nil {
		return protoreflect.ValueOf(nil), false
	}

	data := gokit.MustReturn(proto.Marshal(options))

	options.Reset()
	gokit.Must(proto.UnmarshalOptions{Resolver: extTypes}.Unmarshal(data, options))

	options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() && fd.Name() == optionServiceCode {
			val = v
			return false
		}
		return true
	})

	ok = val.IsValid()
	return
}

func getReplyCode(m *protogen.Method) (code protoreflect.Value, ok bool) {
	options := m.Output.Desc.Options().(*descriptorpb.MessageOptions)
	if options == nil {
		return
	}

	data := gokit.MustReturn(proto.Marshal(options))

	options.Reset()
	gokit.Must(proto.UnmarshalOptions{Resolver: extTypes}.Unmarshal(data, options))

	options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if fd.IsExtension() && fd.Name() == optionReplyCode {
			code = v
			return false
		}
		return true
	})

	ok = code.IsValid()
	return
}
