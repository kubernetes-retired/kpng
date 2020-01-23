package localnetv1

func ParseProtocol(s string) Protocol {
	return Protocol(Protocol_value[s]) // default is Unknown
}
