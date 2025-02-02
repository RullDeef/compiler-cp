package main

import "fmt"

type ListNode struct {
	Next  *ListNode
	Value int
}

func traverse_rec(node *ListNode) {
	if node == nil {
		return
	}
	defer traverse_rec(node.Next)
	fmt.Printf("[%d] -> ", node.Value)
}

func traverse(head *ListNode) {
	defer fmt.Printf("null\n")
	traverse_rec(head)
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

func initList(nodes *[5]ListNode) {
	for i := 0; i < 4; i++ {
		nodes[i].Value = 5 * i
		nodes[i].Next = &(*nodes)[i+1]
	}
	nodes[4].Value = 20
}

func main() {
	var nodes [5]ListNode
	initList(&nodes)

	fmt.Printf("initial list:\n")
	traverse(&nodes[0])

	rev := reverse(&nodes[0])

	fmt.Printf("reversed list:\n")
	traverse(rev)
}
