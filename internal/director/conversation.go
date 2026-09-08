package director

import "time"

func conversationFocus(position time.Duration) string {
	if position < 15*time.Second {
		return "简短开场即可，不虚构今天经历，不连续欢迎。"
	}
	return "沿着上一段动作发展一个具体可见的变化，优先落实观众指定的行动和场景；台词可留白，不反复提问，不安排看弹幕。"
}
