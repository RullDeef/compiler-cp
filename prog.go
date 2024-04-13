package main

type ListNode struct {
	Next  *ListNode
	Value int
}

func traverse(node *ListNode) {
	if node != nil {
		printf("[%d] -> ", (*node).Value)
		traverse((*node).Next)
	}
}

func main() {
	var nodes [5]ListNode

	for i := 0; i < 4; i++ {
		nodes[i].Value = 5 * i
		nodes[i].Next = &nodes[i+1]
	}
	nodes[4].Value = 20

	traverse(&nodes[0])
}
