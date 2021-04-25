package util

type Node struct {
	Next  *Node
	Value interface{}
}

type List struct {
	Head *Node
	tail *Node
}

func (list *List) Insert(key interface{}) {
	node := &Node{
		Value: key,
	}

	if list.tail == nil {
		list.Head = node
	} else {
		list.tail.Next = node
	}

	list.tail = node
}
