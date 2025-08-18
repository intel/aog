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
			// Received stop signal, exit animation loop
			fmt.Printf("\r%s completed!            \n", msg)
			return
		default:
			// Print current animation character
			fmt.Printf("\r%s...  %c", msg, animations[animationIndex][charIndex])
			// Move to next animation character
			charIndex = (charIndex + 1) % len(animations[animationIndex])
			// Switch animation style after a period of time
			if charIndex == 0 {
				animationIndex = (animationIndex + 1) % len(animations)
			}
			// Pause for a while to control animation speed
			time.Sleep(150 * time.Millisecond)
		}
	}
}
