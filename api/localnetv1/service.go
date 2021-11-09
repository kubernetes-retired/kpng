package localnetv1

func (s *Service) NamespacedName() string {
	return s.Namespace + "/" + s.Name
}
