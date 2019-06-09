package httprouter

// CleanPath函数是URL版本的path.Clean，返回去除多于的.和..的简洁路径
//
// 以下处理规则迭代重复的进行处理直到没有可处理的:
//	1. 将多个斜杠替代为单斜杠
//	2. 去除每个 . 路径元素
//	3. 去除每个 .. 路径元素，如果前面存在元素，一起替换，例如：/static/css/../style.css 被替换为 /static/style.css
//	4. 去除根路径的 .. 元素，例如：/../ 被替换为 /
//
// 如果处理结束之后路径是空字符串返回'/'
func CleanPath(p string) string {
	if p == "" {
		return "/"
	}

	n := len(p)
	var buf []byte

	// r 是处理请求路径的下一个字节的索引
	r := 1
	// w 是写入buf缓冲区的下一个字节的索引
	w := 1

	// 判断路径是否以'/'开始，否则自动处理为'/'开始
	if p[0] != '/' {
		r = 0
		buf = make([]byte, n+1)
		buf[0] = '/'
	}

	// 判断路径是否以'/'结束
	trailing := n > 1 && p[n-1] == '/'

	// 顺序读取请求路径的每个字符
	// 假设路径为：/src/.././//filepath/，应该返回 /src/filepath/
	for r < n {
		switch {
		case p[r] == '/':
			// empty path element, trailing slash is added after the end
			r++

		case p[r] == '.' && r+1 == n:
			trailing = true
			r++

		case p[r] == '.' && p[r+1] == '/':
			// . element
			r += 2

		case p[r] == '.' && p[r+1] == '.' && (r+2 == n || p[r+2] == '/'):
			// .. element: remove to last /
			r += 3

			if w > 1 {
				// can backtrack
				w--

				if buf == nil {
					for w > 1 && p[w] != '/' {
						w--
					}
				} else {
					for w > 1 && buf[w] != '/' {
						w--
					}
				}
			}

		default:
			// real path element.
			// add slash if needed
			if w > 1 {
				bufApp(&buf, p, w, '/')
				w++
			}

			// copy element
			for r < n && p[r] != '/' {
				bufApp(&buf, p, w, p[r])
				w++
				r++
			}
		}
	}

	// re-append trailing slash
	if trailing && w > 1 {
		bufApp(&buf, p, w, '/')
		w++
	}

	if buf == nil {
		return p[:w]
	}

	return string(buf[:w])
}

func bufApp(buf *[]byte, s string, w int, c byte) {
	if *buf == nil {
		if s[w] == c {
			return
		}

		*buf = make([]byte, len(s))
		copy(*buf, s[:w])
	}
	(*buf)[w] = c
}
