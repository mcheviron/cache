package cache

type Node[T any] struct {
	next  *Node[T]
	prev  *Node[T]
	value T
}

func newNode[T any](value T) *Node[T] {
	return &Node[T]{value: value}
}

type queue[T any] struct {
	head *Node[T]
	tail *Node[T]
}

func (q *queue[T]) pushToFront(value T) {
	n := newNode(value)
	if q.head == nil {
		q.head = n
		q.tail = n
		return
	}
	n.next = q.head
	q.head.prev = n
	q.head = n
}

func (q *queue[T]) remove(node *Node[T]) {
	if node == nil {
		return
	}
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		q.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		q.tail = node.prev
	}

	// make sure they're gc'd
	node.next = nil
	node.prev = nil
}

func (q *queue[T]) moveToFront(node *Node[T]) {
	if node == nil || q.head == node {
		return
	}

	if node.prev != nil {
		node.prev.next = node.next
	} else {
		q.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		q.tail = node.prev
	}

	node.next = q.head
	node.prev = nil
	if q.head != nil {
		q.head.prev = node
	}
	q.head = node
	if q.tail == nil {
		q.tail = node
	}
}
