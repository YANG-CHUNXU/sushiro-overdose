package app

import (
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 前端契约守卫：web_static.go 里的 indexHTML 是一个内嵌 HTML/JS 字符串，go build
// 只验证它是合法 Go 字符串、不校验内容。下面的测试对其做静态结构检查，让"重排丢
// id、删函数后 onclick 还在调、id 撞车、JS 语法错"这类回归在 go test 阶段就报红，
// 而不是等用户打开页面才暴露。检查逻辑与 architecture_guard_test.go 同一风格（静态
// 扫描源码字符串，零外部依赖）。

func extractEmbeddedScript(t *testing.T) string {
	t.Helper()
	const open, closeTag = "<script>", "</script>"
	i := strings.Index(indexHTML, open)
	j := strings.LastIndex(indexHTML, closeTag)
	if i < 0 || j < 0 || j <= i {
		t.Fatalf("indexHTML 中找不到成对的 <script> 段")
	}
	return indexHTML[i+len(open) : j]
}

// satisfiedDOMIDs 收集所有"存在的" id：模板里的 id="X"，以及 JS 里动态创建的 .id='X'。
func satisfiedDOMIDs() map[string]bool {
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`id="([\w-]+)"`).FindAllStringSubmatch(indexHTML, -1) {
		ids[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`\.id\s*=\s*['"]([\w-]+)['"]`).FindAllStringSubmatch(indexHTML, -1) {
		ids[m[1]] = true
	}
	return ids
}

// TestEmbeddedDOMIDReferencesResolve 确保 JS 里 el('X') / getElementById('X') 引用的
// 每个静态字面量 id，都能在模板中找到 id="X" 或在 JS 中被动态创建。挡住"重排/改名丢 id"。
func TestEmbeddedDOMIDReferencesResolve(t *testing.T) {
	satisfied := satisfiedDOMIDs()
	refRe := []*regexp.Regexp{
		regexp.MustCompile(`\bel\('([\w-]+)'\)`),
		regexp.MustCompile(`getElementById\('([\w-]+)'\)`),
	}
	missing := map[string]bool{}
	for _, re := range refRe {
		for _, m := range re.FindAllStringSubmatch(indexHTML, -1) {
			if !satisfied[m[1]] {
				missing[m[1]] = true
			}
		}
	}
	if len(missing) > 0 {
		t.Fatalf("JS 引用了不存在的 DOM id（模板里没有 id=\"...\"、JS 里也没动态创建）：%s\n"+
			"如果是重排或改名导致，请补回对应元素 id；如确为动态创建，请用 element.id='...' 赋值。", sortedKeys(missing))
	}
}

// TestEmbeddedOnclickHandlersDefined 确保每个 onclick="fn(...)" 的首个调用函数 fn 在
// 脚本里有定义（function 声明或赋值/箭头）。挡住"删了/改名了函数但 HTML 还在调"。
func TestEmbeddedOnclickHandlersDefined(t *testing.T) {
	js := extractEmbeddedScript(t)
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`function\s+([a-zA-Z_$][\w$]*)\s*\(`).FindAllStringSubmatch(js, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`\b([a-zA-Z_$][\w$]*)\s*=\s*(?:async\s+)?(?:function|\()`).FindAllStringSubmatch(js, -1) {
		defined[m[1]] = true
	}

	leadCall := regexp.MustCompile(`^\s*([a-zA-Z_$][\w$]*)\(`)
	undef := map[string]bool{}
	for _, m := range regexp.MustCompile(`onclick="([^"]*)"`).FindAllStringSubmatch(indexHTML, -1) {
		lc := leadCall.FindStringSubmatch(m[1])
		if lc == nil {
			continue // 非直接函数调用（赋值、成员表达式等）不在此检查范围
		}
		if !defined[lc[1]] {
			undef[lc[1]] = true
		}
	}
	if len(undef) > 0 {
		t.Fatalf("onclick 调用了脚本中未定义的函数：%s\n"+
			"如果改名/删除了函数，请同步更新对应 onclick。", sortedKeys(undef))
	}
}

// TestEmbeddedDOMIDsUnique 确保模板里没有重复的 id="X"。挡住 id 撞车导致 el() 取错元素。
func TestEmbeddedDOMIDsUnique(t *testing.T) {
	counts := map[string]int{}
	for _, m := range regexp.MustCompile(`id="([\w-]+)"`).FindAllStringSubmatch(indexHTML, -1) {
		counts[m[1]]++
	}
	dups := map[string]bool{}
	for id, n := range counts {
		if n > 1 {
			dups[id] = true
		}
	}
	if len(dups) > 0 {
		t.Fatalf("模板里存在重复 id：%s", sortedKeys(dups))
	}
}

// TestEmbeddedCriticalAnchors 冒烟检查：核心面板/锚点必须存在，防止整页结构被误删。
func TestEmbeddedCriticalAnchors(t *testing.T) {
	satisfied := satisfiedDOMIDs()
	for _, id := range []string{"qdPressChart", "qdAnswer", "qdAdvisor", "qtLive", "qdTargetNo", "ntStore", "snRows", "rc", "lv", "toastWrap", "confirmOv"} {
		if !satisfied[id] {
			t.Errorf("缺少关键锚点 id=%q（模板或动态创建）", id)
		}
	}
	for _, needle := range []string{`name="sushiro-csrf"`, "function toast(", "function confirmDialog("} {
		if !strings.Contains(indexHTML, needle) {
			t.Errorf("indexHTML 缺少关键片段：%s", needle)
		}
	}
}

// TestEmbeddedJavaScriptSyntax 用 node --check 校验内嵌 JS 语法；环境没有 node 时跳过，
// 因此不引入硬依赖。CI 的 runner 自带 node，可作为语法门禁。
func TestEmbeddedJavaScriptSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if node, err = exec.LookPath("nodejs"); err != nil {
			t.Skip("未找到 node，跳过 JS 语法检查")
		}
	}
	js := extractEmbeddedScript(t)
	f, err := os.CreateTemp("", "sushiro-web-*.js")
	if err != nil {
		t.Fatalf("创建临时文件失败：%v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(js); err != nil {
		t.Fatalf("写临时文件失败：%v", err)
	}
	f.Close()
	if out, err := exec.Command(node, "--check", f.Name()).CombinedOutput(); err != nil {
		t.Fatalf("内嵌 JS 未通过 node --check：\n%s", out)
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
