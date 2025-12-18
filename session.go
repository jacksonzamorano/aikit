package aikit

type Session struct {
	Provider InferenceProvider
}

func (s *Session) Start(request InferenceRequest) *ProviderState {
	return s.Provider.Infer(&request, nil)
}

func (s *Session) Stream(request InferenceRequest, onPartial func(*ProviderState)) *ProviderState {
	return s.Provider.Infer(&request, onPartial)
}
