package progress

import (
	"fmt"
	"sync"
	"time"
)

var animations = [][]rune{
	{'▁', '▃', '▄', '▅', '▆', '▇', '█', '▇', '▆', '▅', '▄', '▃', '▁'},
}

func ShowLoadingAnimation(stopChan chan struct{}, wg *sync.WaitGroup, msg string) {
	defer wg.Done()
	animationIndex := 0
	charIndex := 0
	for {
		select {
		case <-stopChan:
			// 接收到停止信号，退出动画循环
			fmt.Printf("\r%s completed!            \n", msg)
			return
		default:
			// 打印当前动画字符
			fmt.Printf("\r%s...  %c", msg, animations[animationIndex][charIndex])
			// 移动到下一个动画字符
			charIndex = (charIndex + 1) % len(animations[animationIndex])
			// 每隔一段时间切换动画样式
			if charIndex == 0 {
				animationIndex = (animationIndex + 1) % len(animations)
			}
			// 暂停一段时间，控制动画速度
			time.Sleep(150 * time.Millisecond)
		}
	}
}
