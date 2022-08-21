package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type Runtime struct {
	input io.Reader
	output io.Writer

	store []byte
	trace bool
	pos int
}

type Pos struct {
	pos int
	lno int
	linepos int
}

type Runner interface {
	Run(rt *Runtime) error
}

type Block struct {
	pos Pos
	seq []Runner
}

func (r *Block) Add(x Runner) {
	r.seq = append(r.seq, x)
}

func (r *Block) Run(rt *Runtime) error {
	if rt.trace {
		fmt.Printf("run %+v at %+v\n", r, r.pos)
	}
	for _, cmd := range r.seq {
		if err := cmd.Run(rt); err != nil {
			return err
		}
	}
	return nil
}

type Loop struct {
	pos Pos
	block *Block
}

func (r *Loop) Run(rt *Runtime) error {
	if rt.trace {
		fmt.Printf("run %+v at %+v\n", r, r.pos)
	}
	for rt.store[rt.pos] != 0 {
		if err := r.block.Run(rt); err != nil {
			return err
		}
	}
	return nil
}

type Move struct {
	pos Pos
	dir int
}

func (r *Move) Run(rt *Runtime) error {
	if rt.trace {
		fmt.Printf("run %+v at %+v\n", r, r.pos)
	}
	rt.pos += r.dir
	if rt.pos < 0 || rt.pos >= len(rt.store) {
		return fmt.Errorf("position %d is out of range at %+v", rt.pos, r.pos)
	}
	return nil
}

type Update struct {
	pos Pos
	dir int8
}

func (r *Update) Run(rt *Runtime) error {
	if rt.trace {
		fmt.Printf("run %+v at %+v\n", r, r.pos)
	}
	rt.store[rt.pos] += byte(r.dir)
	return nil
}

type Getchar struct {
	pos Pos
}

func (r *Getchar) Run(rt *Runtime) error {
	if rt.trace {
		fmt.Printf("run %+v at %+v\n", r, r.pos)
	}
	bs := []byte{0}
	_, err := rt.input.Read(bs)
	if err != nil && err != io.EOF {
		return fmt.Errorf("%v in getchar at %+v", err, r.pos)
	}
	if err == io.EOF {
		bs[0] = 0xff
	}
	rt.store[rt.pos] = bs[0]
	return nil
}

type Putchar struct {
	pos Pos
}

func (r *Putchar) Run(rt *Runtime) error {
	if rt.trace {
		fmt.Printf("run %+v at %+v\n", r, r.pos)
	}
	bs := []byte{ rt.store[rt.pos] }
	_, err := rt.output.Write(bs)
	if err != nil {
		return fmt.Errorf("%v in putchar at %+v", err, r.pos)
	}
	return nil
}

type Parser struct {
	input io.Reader
	pos Pos
	err error
}

func (p *Parser) ParseFile(fn string) (Runner, error) {
	fp, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	return p.Parse(fp)
}

func (p *Parser) Parse(input io.Reader) (Runner, error) {
	p.input = input
	p.pos.lno = 1

	block := &Block{p.pos, []Runner{}}
	p.parseBlock(block, true)
	if p.err != nil {
		return nil, p.err
	}
	return block, nil
}

func (p *Parser) parseBlock(block *Block, top bool) {
	for p.err == nil {
		ch := p.next()
		if ch == 0 {
			break
		}

		switch ch {
		case '<':
			block.Add(&Move{p.pos, -1})
		case '>':
			block.Add(&Move{p.pos, 1})
		case '[':
			inner := &Block{p.pos, []Runner{}}
			p.parseBlock(inner, false)
			block.Add(&Loop{p.pos, inner})
		case ']':
			if top {
				p.err = fmt.Errorf("unexpected close bracket at %+v", p.pos)
			}
			return
		case '+':
			block.Add(&Update{p.pos, 1})
		case '-':
			block.Add(&Update{p.pos, -1})
		case '.':
			block.Add(&Putchar{p.pos})
		case ',':
			block.Add(&Getchar{p.pos})
		default:
			panic("cant happen")
		}
	}
}

// next returns the next valid input byte, or zero for EOF.
// io errors are recorded internally.
func (p *Parser) next() rune {
	for {
		bs := []byte{0}
		_, err := p.input.Read(bs)
		if err != nil {
			//fmt.Printf("parser %v at %+v\n", err, p.pos)
			if err == io.EOF {
				return 0
			}
			p.err = err
			return 0 // XXX rethink
		}

		b := bs[0]
		ch := rune(b)
		//fmt.Printf("parser next %c at %+v\n", ch, p.pos)

		p.pos.pos ++
		p.pos.linepos ++
		if ch == '\n' {
			p.pos.lno ++
			p.pos.linepos = 0
		}
		if strings.Contains("<>+-.,[]", string(ch)) {
			return ch
		}
	}
}

func main() {
	//argv0 := os.Args[0]
	if len(os.Args) != 2 {
		fmt.Printf("usage: prog bf\n")
		return
	}
	fn := os.Args[1]

	parser := Parser{}
	prog, err := parser.ParseFile(fn)
	if err != nil {
		fmt.Printf("%s: %s\n", fn, err)
		return
	}
	
	//fmt.Printf("parsed %+v\n", prog)

	rt := &Runtime{
		input: os.Stdin,
		output: os.Stdout,
		store: make([]byte, 30000),
		//trace: true,
	}
	if err := prog.Run(rt); err != nil {
		fmt.Printf("error %v\n", err)
	}
	return
}
