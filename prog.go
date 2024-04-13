package main

type ListNode struct {
	Next  *ListNode
	Value int
}

func traverse_rec(node *ListNode) {
	if node != nil {
		printf("[%d] -> ", node.Value)
		traverse_rec(node.Next)
	}
}

func traverse(head *ListNode) {
	traverse_rec(head)
	printf("null\n")
}

func reverse(head *ListNode) *ListNode {
	if head == nil {
		return nil
	}
	var newList *ListNode
	for head != nil {
		next := head.Next
		head.Next = newList
		newList = head
		head = next
	}
	return newList
}

func main() {
	var nodes [5]ListNode

	for i := 0; i < 4; i++ {
		nodes[i].Value = 5 * i
		nodes[i].Next = &nodes[i+1]
	}
	nodes[4].Value = 20

	printf("initial list:\n")
	traverse(&nodes[0])

	rev := reverse(&nodes[0])

	printf("reversed list:\n")
	traverse(rev)
}
