package main

import "fmt"

type Node struct {
	Data int
	Next *Node
}

func NewNode(data int) *Node {
	return &Node{
		Data: data,
		Next: nil,
	}
}

func Append(node *Node, data int) *Node {
	if node == nil {
		return NewNode(data)
	} else {
		root := node
		for node.Next != nil {
			node = node.Next
		}
		node.Next = NewNode(data)
		return root
	}
}

func Reverse(node *Node) *Node {
	if node == nil {
		return nil
	}
	var newList *Node
	for node != nil {
		next := node.Next
		node.Next = newList
		newList = node
		node = next
	}
	return newList
}

func PrintList(node *Node) {
	for node != nil {
		fmt.Printf("[%d] ", node.Data)
		node = node.Next
	}
	fmt.Printf("\n")
}

func main() {
	lst := Append(nil, 5)
	lst = Append(lst, 12)
	lst = Append(lst, 20)
	lst = Append(lst, 48)
	lst = Append(lst, 31)

	PrintList(lst)

	lst = Reverse(lst)

	PrintList(lst)
}
