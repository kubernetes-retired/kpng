package localnetv1

func (f *EndpointConditions) Accept(v *EndpointConditions) bool {
	return (!f.Ready || v.Ready) &&
		(!f.Selected || v.Selected) &&
		(!f.Local || v.Local)
}
