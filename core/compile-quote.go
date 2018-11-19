package core

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/MagicalTux/gophp/core/tokenizer"
)

func compileQuoteConstant(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// i.Data is a string such as 'a string' (quotes included)

	if i.Data[0] != '\'' {
		return nil, errors.New("malformed string")
	}
	if i.Data[len(i.Data)-1] != '\'' {
		return nil, errors.New("malformed string")
	}

	in := i.Data[1 : len(i.Data)-1]
	b := &bytes.Buffer{}
	l := len(in)
	loc := MakeLoc(i.Loc())

	for i := 0; i < l; i++ {
		c := in[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i += 1
		if i >= l {
			b.WriteByte(c)
			break
		}
		c = in[i]
		switch c {
		case '\\', '\'':
			b.WriteByte(c)
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}

	return &runZVal{ZString(b.String()), loc}, nil
}

func compileQuoteHeredoc(i *tokenizer.Item, c compileCtx) (Runnable, error) {
	// i == T_START_HEREDOC
	var res runConcat
	var err error

	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		_ = res
		switch i.Type {
		case tokenizer.T_ENCAPSED_AND_WHITESPACE:
			res = append(res, &runZVal{unescapePhpQuotedString(i.Data), MakeLoc(i.Loc())})
		case tokenizer.T_VARIABLE:
			res = append(res, &runVariable{ZString(i.Data[1:]), MakeLoc(i.Loc())})
		case tokenizer.T_END_HEREDOC:
			// end of quote
			return res, nil
		default:
			return nil, i.Unexpected()
		}
	}
}

func compileQuoteEncapsed(i *tokenizer.Item, c compileCtx, q rune) (Runnable, error) {
	// i == '"'

	var res runConcat
	var err error

	for {
		i, err = c.NextItem()
		if err != nil {
			return nil, err
		}

		_ = res
		switch i.Type {
		case tokenizer.T_ENCAPSED_AND_WHITESPACE:
			res = append(res, &runZVal{unescapePhpQuotedString(i.Data), MakeLoc(i.Loc())})
		case tokenizer.T_VARIABLE:
			res = append(res, &runVariable{ZString(i.Data[1:]), MakeLoc(i.Loc())})
		case tokenizer.ItemSingleChar:
			switch []rune(i.Data)[0] {
			case q:
				// end of quote
				return res, nil
			}
		default:
			return nil, i.Unexpected()
		}
	}
}

func unescapePhpQuotedString(in string) ZString {
	t := &bytes.Buffer{}

	for len(in) > 0 {
		if in[0] != '\\' {
			t.WriteByte(in[0])
			in = in[1:]
			continue
		}
		if len(in) == 1 {
			// end of string
			t.WriteByte(in[0])
			break
		}
		in = in[1:]

		switch in[0] {
		case 't':
			t.WriteByte('\t')
		case 'n':
			t.WriteByte('\n')
		case 'v':
			t.WriteByte('\v')
		case 'f':
			t.WriteByte('\f')
		case 'r':
			t.WriteByte('\r')
		case '"', '\\':
			t.WriteByte(in[0])
		case '0', '1', '2', '3', '4', '5', '6', '7':
			t.WriteByte(in[0] - '0')
		case 'x':
			if len(in) < 3 {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			i, err := strconv.ParseUint(in[1:3], 16, 8)
			if err != nil {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			t.WriteByte(byte(i))
			in = in[2:]
		case 'u':
			if len(in) < 3 || in[1] != '{' {
				// too short
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			pos := strings.IndexByte(in, '}')
			if pos == -1 {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			i, err := strconv.ParseUint(in[2:pos], 16, 64)
			if err != nil {
				t.WriteByte('\\')
				t.WriteByte(in[0])
				break
			}
			t.WriteRune(rune(i))
			in = in[pos:]
		default:
			t.WriteByte('\\')
			t.WriteByte(in[0])
		}
		in = in[1:]
	}

	return ZString(t.String())
}
