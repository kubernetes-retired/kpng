package localnetv1

type OpSink interface {
	Send(op *OpItem) error
}
