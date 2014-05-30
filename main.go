package main

import (
	"bytes"
	"code.google.com/p/go.net/html"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
)

type treeNode struct {
	Data     string
	Children []*treeNode
	Parent   *treeNode
	Type     string
	Score    float64
	Attrs    map[string]string
	Position int
}

// Stack implementation, to keep all Nodes in the right order

type Stack struct {
	top  *Element
	size int
}

type Element struct {
	value *treeNode
	next  *Element
}

// Return the stack's length
func (s *Stack) Len() int {
	return s.size
}

// Push a new element onto the stack
func (s *Stack) Push(value *treeNode) {
	s.top = &Element{value, s.top}
	s.size++
}

// Remove the top element from the stack and return it's value
// If the stack is empty, return nil
func (s *Stack) Pop() (value *treeNode) {
	if s.size > 0 {
		value, s.top = s.top.value, s.top.next
		s.size--
		return
	}
	return &treeNode{}
}

// Return the value of the element on the top of the stack
// but don't remove it. If the stack is empty, return nil
func (s *Stack) Peek() (value *treeNode) {
	if s.size > 0 {
		value = s.top.value
		return
	}
	return &treeNode{}
}

var stack *Stack

// Node Methods

// Get the Text content of each node Recursively
func (n *treeNode) Text() string {

	var textBuffer bytes.Buffer

	if elementsToIgnore[n.Type] {
		return ""
	}
	if len(n.Children) < 1 {
		textBuffer.WriteString(" " + n.Data)
	}

	end := len(n.Data)
	start := 0
	for i := 0; i < len(n.Children); i += 1 {
		newText := n.Children[i].Text()

		if i+1 < len(n.Children) {
			end = n.Children[i+1].Position
		} else {
			end = len(n.Data)
		}
		if i > 0 {
			start = n.Children[i].Position
		}

		textBuffer.WriteString(n.Data[start:n.Children[i].Position])
		textBuffer.WriteString(newText)
		textBuffer.WriteString(n.Data[n.Children[i].Position:end])
	}
	textBuffer.WriteString(" ")
	return textBuffer.String()
}

// Get the Html content of each node recursively
func (n *treeNode) Html() string {
	if elementsToIgnore[n.Type] {
		return ""
	}
	if n.Type == "root" {
		return n.Children[0].Html()
	}
	var htmlBuffer bytes.Buffer

	htmlBuffer.WriteString(" <")
	htmlBuffer.WriteString(n.Type)
	for k, v := range n.Attrs {
		htmlBuffer.WriteString(" ")
		htmlBuffer.WriteString(k)
		htmlBuffer.WriteString("='")
		htmlBuffer.WriteString(v)
		htmlBuffer.WriteString("'")
	}

	htmlBuffer.WriteString(">")
	if len(n.Children) == 0 {
		htmlBuffer.WriteString(n.Data)
	} else {
		end := len(n.Data)
		start := 0
		for i := 0; i < len(n.Children); i += 1 {
			newHtml := n.Children[i].Html()

			if i+1 < len(n.Children) {
				end = n.Children[i+1].Position
			} else {
				end = len(n.Data)
			}
			if i > 0 {
				start = n.Children[i].Position
			}

			htmlBuffer.WriteString(n.Data[start:n.Children[i].Position])
			htmlBuffer.WriteString(newHtml)
			htmlBuffer.WriteString(n.Data[n.Children[i].Position:end])

		}
	}
	htmlBuffer.WriteString("</" + n.Type + "> ")

	return htmlBuffer.String()
}

// remove a node and all of its children
// Should be no memory leaks here
func (n *treeNode) Remove() {
	parent := n.Parent
	if len(parent.Children) == 1 {
		parent.Children = []*treeNode{}
		return
	}
	for i := 0; i < len(parent.Children); i += 1 {
		if parent.Children[i] == n {
			copy(parent.Children[i:], parent.Children[i+1:])
			parent.Children[len(parent.Children)-1] = nil // or the zero value of T
			parent.Children = parent.Children[:len(parent.Children)-1]
		}
	}

}

// Calculate the Link density of the node
func (n *treeNode) LinkDensity() int {
	links := n.FindByType("a")
	length := len(n.Text()) + 1
	linkLength := 1
	for i := 0; i < len(links); i += 1 {
		if val, ok := n.Attrs["href"]; ok {
			fmt.Printf("%#v", val)
			linkLength = linkLength + len(val)

		}
	}
	return linkLength / length

}

// Find Node by Type. Case sensitive
func (n *treeNode) FindByType(t string) []*treeNode {
	result := []*treeNode{}
	if n.Type == t {
		result = append(result, n)
	}
	if len(n.Children) < 1 {
		return result
	}
	for i := 0; i < len(n.Children); i += 1 {
		result = append(result, n.Children[i].FindByType(t)...)
	}

	return result
}

// Find Node by Class. Case sensitive
func (n *treeNode) FindByClass(class string) []*treeNode {
	nClass := ""
	result := []*treeNode{}

	if val, ok := n.Attrs["class"]; ok {
		nClass = val
	}
	if nClass == class {
		result = append(result, n)
	}
	if len(n.Children) < 1 {
		return result
	}
	for i := 0; i < len(n.Children); i += 1 {
		result = append(result, n.Children[i].FindByClass(class)...)
	}

	return result
}

// Elements that are assumed to have no Data in them.
var voidElements = map[string]bool{
	"meta":  true,
	"br":    true,
	"link":  true,
	"input": true,
	"hr":    true,
	"img":   true,
}

// Elements that will be ignored when parsing the Html
var elementsToIgnore = map[string]bool{
	"meta":     true,
	"iframe":   true,
	"noscript": true,
	"script":   true,
	"style":    true,
	"aside":    true,
	"object":   true,
}

// Weights of different types of nodes based on their content value
var nodeTypes = map[string]float64{"div": 5, "pre": 3, "td": 3, "blockquote": 3, "address": -3, "ol": -3, "ul": -3,
	"dl": -3, "dd": -3, "dt": -3, "li": -3, "h1": -5,
	"h2": -5, "h3": -5, "h4": -5, "h5": -5, "h6": -5, "th": -6,
}

// General Regexps
var regexps = map[string]*regexp.Regexp{
	"unlikelyCandidates":     regexp.MustCompile("/combx|pager|comment|disqus|foot|header|menu|meta|nav|rss|shoutbox|sidebar|sponsor|share|bookmark|social|advert|leaderboard|instapaper_ignore|entry-unrelated/i"),
	"okMaybeItsACandidateRe": regexp.MustCompile("/and|article|body|column|main/i"),
	"positiveRe":             regexp.MustCompile("/article|body|content|entry|hentry|page|pagination|post|text/i"),
	"negativeRe":             regexp.MustCompile("/combx|comment|captcha|contact|foot|footer|footnote|link|media|meta|promo|related|scroll|shoutbox|sponsor|utility|tags|widget|tip|dialog/i"),
	"divToPElementsRe":       regexp.MustCompile("/<(a|blockquote|dl|div|img|ol|p|pre|table|ul)/i"),
	"replaceBrsRe":           regexp.MustCompile("/(<br[^>]*>[ \n\r\t]*){2,}/gi"),
	"replaceFontsRe":         regexp.MustCompile("/<(/?)font[^>]*>/gi"),
	"trimRe":                 regexp.MustCompile("/^s+|s+$/g"),
	"normalizeRe":            regexp.MustCompile("/s{2,}/g"),
	"killBreaksRe":           regexp.MustCompile("/(<brs*/?>(s|&nbsp;?)*){1,}/g"),
	"videoRe":                regexp.MustCompile("/http://(www.)?(youtube|vimeo|youku|tudou|56|yinyuetai).com/i"),
	"commas":                 regexp.MustCompile("[,，.。;；]"),
}

var topNode *treeNode

func getPage(url string) io.ReadCloser {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Get(url)
	if err != nil {
		panic(err)
	}

	return res.Body
}

func parseNode(tokenizer *html.Tokenizer) bool {
	tokenType := tokenizer.Next()
	d := tokenizer.Token()
	parent := stack.Peek()

	switch tokenType {
	case html.ErrorToken:
		return true
	case html.StartTagToken: // <tag>
		if !voidElements[d.Data] {
			child := treeNode{"", []*treeNode{}, parent, d.Data, 0, map[string]string{}, len(parent.Data)}
			// Save the attributes
			for i := 0; i < len(d.Attr); i++ {
				current := d.Attr[i]
				child.Attrs[current.Key] = current.Val
			}

			// Score the node based on classname
			initializeNode(&child)

			parent.Children = append(parent.Children, &child)
			stack.Push(&child)
		}
		return parseNode(tokenizer)
	case html.TextToken:
		// Ignore the empty nodes
		if len(d.Data) > 1 {
			parent.Data = parent.Data + strings.TrimSpace(d.Data)
		}
		return parseNode(tokenizer)
	case html.EndTagToken: // </tag>
		classAndId := ""

		if val, ok := parent.Attrs["class"]; ok {
			classAndId += val
		}

		if val, ok := parent.Attrs["id"]; ok {
			classAndId += val
		}

		if parent.Type == "p" || parent.Type == "pre" {
			commas := regexps["commas"].FindAllString(parent.Text(), 15)
			score := float64(len(commas)) + math.Min(float64(len(parent.Text()))/100, 3)

			if parent.Parent != nil {
				parent.Parent.Score = parent.Parent.Score + score
			}

			if parent.Parent != nil && parent.Parent.Parent != nil {
				parent.Parent.Parent.Score = parent.Parent.Parent.Score + score/2
			}
		}
		stack.Pop()

		// Remove unlikely nodes
		if (parent.Type == "head" || parent.Type == "footer") || regexps["unlikelyCandidates"].MatchString(classAndId) && !regexps["okMaybeItsACandidateRe"].MatchString(classAndId) {
			parent.Remove()
		}
		return parseNode(tokenizer)
	case html.SelfClosingTagToken: // <tag/>
		if !voidElements[d.Data] {
			stack.Pop()
		}
		return parseNode(tokenizer)
	default:
		return parseNode(tokenizer)
	}
}

// Score the node based on Class, ID and type of node
func initializeNode(n *treeNode) {
	if val, ok := nodeTypes[n.Type]; ok {
		n.Score = n.Score + val
		classAndId := ""
		if val, ok := n.Attrs["class"]; ok {
			classAndId += val
		}
		if val, ok := n.Attrs["id"]; ok {
			classAndId += val
		}
		if regexps["negativeRe"].MatchString(classAndId) {
			n.Score = n.Score - 25
		}
		if regexps["positiveRe"].MatchString(classAndId) {
			n.Score = n.Score + 25
		}
		if n.Type == "article" {
			n.Score = n.Score + 25
		}
	}
}

// Main method. Tokenize the html and parse it. Returns the root node
func parseHtml(r io.ReadCloser) *treeNode {
	defer r.Close()
	d := html.NewTokenizer(r)
	node := treeNode{"", []*treeNode{}, nil, "root", 0, map[string]string{}, 0}
	stack.Push(&node)
	parseNode(d)
	return &node
}

// Traverse the tree
func traverse(node *treeNode) {
	if len(node.Children) < 1 {
		return
	}
	if topNode == nil {
		topNode = node
	}
	if topNode.Score < node.Score {
		topNode = node
	}

	for i := 0; i < len(node.Children); i += 1 {
		traverse(node.Children[i])
	}
}

// Testing
func main() {
	stack = new(Stack)
	page := getPage("http://www.theguardian.com/world/2014/apr/27/ukraine-kidnapped-observers-slavyansk-vyacheslav-ponomarev")
	root := parseHtml(page)
	traverse(root)
	fmt.Printf("%#v\n", topNode.Text())
	//root.Html()
}
