package main

import (
	"github.com/joyparty/gokit"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	nhPackage      = protogen.GoImportPath("github.com/joyparty/nodehub/proto/nh")
	protoPackage   = protogen.GoImportPath("google.golang.org/protobuf/proto")
	reflectPackage = protogen.GoImportPath("reflect")
)

type Message struct {
	*protogen.Message
	ReplyService protoreflect.Value
	ReplyCode    protoreflect.Value
}

func genPackFunctions(file *protogen.File, g *protogen.GeneratedFile) bool {
	messages := parseMessages(file)

	lo.ForEach(messages, func(m Message, _ int) {
		g.P()
		g.P("func Pack", m.GoIdent, "(msg *", m.GoIdent, ") (*", nhPackage.Ident("Reply"), ", error) {")
		g.P("data, err := ", protoPackage.Ident("Marshal"), "(msg)")
		g.P("if err != nil { return nil, err}")
		g.P()
		g.P("return &", nhPackage.Ident("Reply"), "{")
		g.P("ServiceCode:", m.ReplyService.Interface(), ",")
		g.P("Code:", m.ReplyCode.Interface(), ",")
		g.P("Data: data,")
		g.P("}, nil")
		g.P("}")
	})

	return len(messages) > 0
}

func genReplyMessages(file *protogen.File, g *protogen.GeneratedFile) bool {
	if !config.ReplyMessages {
		return false
	}

	services := parseServices(file)
	messages := parseMessages(file)
	if len(services) == 0 && len(messages) == 0 {
		return false
	}

	done := map[protogen.GoIdent]struct{}{}

	g.P()
	g.P("// ReplyMessages 所有的返回值类型及其编码 [2]int32{service_code, reply_code}")
	g.P("var ReplyMessages = map[[2]int32]", reflectPackage.Ident("Type"), "{")
	lo.ForEach(services, func(s Service, _ int) {
		lo.ForEach(s.Methods, func(m Method, _ int) {
			messageType := m.Output.GoIdent
			if _, ok := done[messageType]; !ok {
				g.P("{", s.Code.Interface(), ",", m.ReplyCode.Interface(), "}: ", reflectPackage.Ident("TypeOf"), "(", messageType, "{}),")
				done[messageType] = struct{}{}
			}
		})
	})

	lo.ForEach(messages, func(m Message, _ int) {
		messageType := m.GoIdent
		if _, ok := done[messageType]; !ok {
			g.P("{", m.ReplyService.Interface(), ",", m.ReplyCode.Interface(), "}: ", reflectPackage.Ident("TypeOf"), "(", messageType, "{}),")
			done[messageType] = struct{}{}
		}
	})
	g.P("}")

	return true
}

func parseMessages(file *protogen.File) []Message {
	return lo.Filter(
		lo.Map(file.Messages, func(m *protogen.Message, _ int) Message {
			options := m.Desc.Options().(*descriptorpb.MessageOptions)
			if options == nil {
				return Message{}
			}

			data := gokit.MustReturn(proto.Marshal(options))
			options.Reset()
			gokit.Must(proto.UnmarshalOptions{Resolver: extTypes}.Unmarshal(data, options))

			var replyService, replyCode protoreflect.Value
			options.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
				if fd.IsExtension() {
					switch fd.Name() {
					case optionReplyService:
						replyService = v
					case optionReplyCode:
						replyCode = v
					}
				}
				return true
			})

			return Message{
				Message:      m,
				ReplyService: replyService,
				ReplyCode:    replyCode,
			}
		}),
		func(m Message, _ int) bool {
			return m.ReplyService.IsValid() && m.ReplyCode.IsValid()
		},
	)
}
