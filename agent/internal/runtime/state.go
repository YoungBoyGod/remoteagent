package runtime

import (
	"fmt"
	"log"
)

// validTransitions 定义合法的状态转换规则表。
// key 为当前状态，value 为该状态允许转换到的目标状态列表。
var validTransitions = map[State][]State{
	StateInit:        {StateInit, StateRegistering},
	StateRegistering: {StateRunning, StateStopped},
	StateRunning:     {StateAuthExpired, StateDraining, StateStopped},
	StateAuthExpired: {StateRegistering, StateStopped},
	StateDraining:    {StateStopped},
	StateStopped:     {},
}

// isValidTransition 检查从 from 状态到 to 状态的转换是否合法
func isValidTransition(from, to State) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

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

// setState 设置 Agent 状态，设置前会校验状态转换是否合法。
// 非法转换时记录警告日志并返回错误，状态不会被修改。
func (a *Agent) setState(state State) error {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	if !isValidTransition(a.state, state) {
		err := fmt.Errorf("非法状态转换: %s -> %s", a.state, state)
		log.Printf("[WARN] %v", err)
		return err
	}
	a.state = state
	return nil
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
