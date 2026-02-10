package runtime

func (a *Agent) requestShutdown(reason string) {
	select {
	case a.shutdownCh <- reason:
	default:
	}
}

func (a *Agent) triggerReauth() {
	select {
	case a.reauthCh <- struct{}{}:
	default:
	}
}

func (a *Agent) setState(state State) {
	a.stateMu.Lock()
	a.state = state
	a.stateMu.Unlock()
}

func (a *Agent) getState() State {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.state
}

func (a *Agent) getToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.token
}
