package main

import (
	"fmt"
	"strings"

	"github.com/joyparty/gokit"
	"github.com/samber/lo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	nhPackage    = protogen.GoImportPath("github.com/joyparty/nodehub/proto/nh")
	protoPackage = protogen.GoImportPath("google.golang.org/protobuf/proto")
)

type Message struct {
	*protogen.Message
	ReplyService protoreflect.Value
	ReplyCode    protoreflect.Value
}

func genPackMessages(file *protogen.File, g *protogen.GeneratedFile) bool {
	messages := lo.Filter(
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
					if strings.HasSuffix(string(fd.FullName()), fmt.Sprintf(".%s", optionReplyService)) {
						replyService = v
					} else if strings.HasSuffix(string(fd.FullName()), fmt.Sprintf(".%s", optionReplyCode)) {
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
