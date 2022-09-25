package localnetv1

// Any returns true iff there's any scope at true
func (s *EndpointScopes) Any() bool {
	return s.Internal ||
		s.External
}
