package promsketch

import (
	"fmt"
	"testing"
)

func TestTopKHeap(t *testing.T) {
	topkheap := NewTopKHeap(5)

	topkheap.Update("orange", 1)
	fmt.Println("after insert orange:")
	topkheap.Print()

	topkheap.Update("mango", 10)
	fmt.Println("after insert mango:")
	topkheap.Print()

	topkheap.Insert("lemon", 8)
	fmt.Println("after insert lemon:")
	topkheap.Print()

	topkheap.Insert("kiwi", 11)
	fmt.Println("after insert kiwi:")
	topkheap.Print()

	topkheap.Update("aa", 1)
	fmt.Println("after insert aa:")
	topkheap.Print()
	topkheap.Update("mango", 0)
	fmt.Println("after update mongo:")
	topkheap.Print()

	topkheap.Update("mango", 100)
	fmt.Println("after update mongo:")
	topkheap.Print()

	topkheap.Update("bb", 3)
	fmt.Println("after update bb:")
	topkheap.Print()

	topkheap.Update("cc", 5)
	fmt.Println("after update cc:")
	topkheap.Print()

	topkheap.Update("dd", 1)
	fmt.Println("after update dd:")
	topkheap.Print()
}
