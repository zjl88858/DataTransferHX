package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"filetransferhx/config"
	"filetransferhx/core"
)

const (
	configPath  = "config.toml"
	historyPath = "history.json"
)

// TOML语法高亮颜色
var (
	colorComment = color.NRGBA{R: 106, G: 153, B: 85, A: 255}  // 绿色 - 注释
	colorKey     = color.NRGBA{R: 156, G: 220, B: 254, A: 255} // 浅蓝 - 键名
	colorValue   = color.NRGBA{R: 206, G: 145, B: 120, A: 255} // 橙色 - 字符串值
	colorNumber  = color.NRGBA{R: 181, G: 206, B: 168, A: 255} // 浅绿 - 数字
	colorSection = color.NRGBA{R: 197, G: 134, B: 192, A: 255} // 紫色 - 节标题
	colorNormal  = color.NRGBA{R: 212, G: 212, B: 212, A: 255} // 白色 - 普通文本
)

// 正则表达式
var (
	reSection = regexp.MustCompile(`^\s*\[\[?[^\]]+\]\]?\s*$`)
	reKeyVal  = regexp.MustCompile(`^(\s*)([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(.*)$`)
	reNumber  = regexp.MustCompile(`^-?\d+\.?\d*$`)
)

// LogWriter 自定义日志写入器
type LogWriter struct {
	mu     sync.Mutex
	buffer strings.Builder
}

func NewLogWriter() *LogWriter {
	return &LogWriter{}
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	os.Stdout.Write(p)
	w.buffer.Write(p)
	return len(p), nil
}

func (w *LogWriter) GetText() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

// AppState 应用状态管理
type AppState struct {
	mu      sync.RWMutex
	running bool
	runner  *core.Runner
	hm      *core.HistoryManager
}

func (s *AppState) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ClickableHighlightView 可点击的高亮视图
type ClickableHighlightView struct {
	widget.BaseWidget
	content   *fyne.Container
	onTapped  func()
	bgRect    *canvas.Rectangle
	isHovered bool
}

func NewClickableHighlightView(onTapped func()) *ClickableHighlightView {
	c := &ClickableHighlightView{
		onTapped: onTapped,
		bgRect:   canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 30, A: 255}),
		content:  container.NewVBox(),
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *ClickableHighlightView) SetContent(lines []fyne.CanvasObject) {
	c.content.RemoveAll()
	for _, line := range lines {
		c.content.Add(line)
	}
	c.content.Refresh()
}

func (c *ClickableHighlightView) Tapped(_ *fyne.PointEvent) {
	if c.onTapped != nil {
		c.onTapped()
	}
}

func (c *ClickableHighlightView) TappedSecondary(_ *fyne.PointEvent) {}

func (c *ClickableHighlightView) MouseIn(_ *desktop.MouseEvent) {
	c.isHovered = true
	c.bgRect.FillColor = color.NRGBA{R: 40, G: 40, B: 40, A: 255}
	c.bgRect.Refresh()
}

func (c *ClickableHighlightView) MouseMoved(_ *desktop.MouseEvent) {}

func (c *ClickableHighlightView) MouseOut() {
	c.isHovered = false
	c.bgRect.FillColor = color.NRGBA{R: 30, G: 30, B: 30, A: 255}
	c.bgRect.Refresh()
}

func (c *ClickableHighlightView) CreateRenderer() fyne.WidgetRenderer {
	stack := container.NewStack(c.bgRect, c.content)
	return widget.NewSimpleRenderer(stack)
}

func main() {
	// 在创建应用之前设置字体环境变量
	if _, err := os.Stat("font.ttf"); err == nil {
		os.Setenv("FYNE_FONT", "font.ttf")
	}

	myApp := app.New()
	myWindow := myApp.NewWindow("数据传输系统HX-v1.0.0")

	state := &AppState{}

	// ========== 左侧：配置编辑区 ==========
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		configContent = []byte("# 配置文件加载失败")
	}
	currentText := string(configContent)

	// 编辑器
	configEditor := widget.NewMultiLineEntry()
	configEditor.SetText(currentText)
	configEditor.Wrapping = fyne.TextWrapOff
	configEditor.TextStyle = fyne.TextStyle{Monospace: true}

	editorScroll := container.NewScroll(configEditor)

	// 高亮视图容器
	var highlightView *ClickableHighlightView
	var switchContainer *fyne.Container
	isEditing := false

	// 创建高亮行
	createHighlightLines := func(text string) []fyne.CanvasObject {
		var lines []fyne.CanvasObject
		for _, line := range strings.Split(text, "\n") {
			lines = append(lines, createHighlightedLine(line))
		}
		return lines
	}

	// 切换到编辑模式
	switchToEdit := func() {
		if isEditing {
			return
		}
		isEditing = true
		configEditor.SetText(currentText)
		switchContainer.RemoveAll()
		switchContainer.Add(editorScroll)
		switchContainer.Refresh()
		// 聚焦编辑器
		myWindow.Canvas().Focus(configEditor)
	}

	// 切换到预览模式
	switchToPreview := func() {
		isEditing = false
		currentText = configEditor.Text
		highlightView.SetContent(createHighlightLines(currentText))
		switchContainer.RemoveAll()
		highlightScroll := container.NewScroll(highlightView)
		switchContainer.Add(highlightScroll)
		switchContainer.Refresh()
	}

	// 初始化高亮视图
	highlightView = NewClickableHighlightView(switchToEdit)
	highlightView.SetContent(createHighlightLines(currentText))

	highlightScroll := container.NewScroll(highlightView)
	switchContainer = container.NewMax(highlightScroll)

	// 保存按钮
	saveBtn := widget.NewButtonWithIcon("保存配置", theme.DocumentSaveIcon(), func() {
		if isEditing {
			currentText = configEditor.Text
		}
		err := os.WriteFile(configPath, []byte(currentText), 0644)
		if err != nil {
			dialog.ShowError(fmt.Errorf("保存失败: %v", err), myWindow)
			return
		}
		// 保存后切换回预览模式
		switchToPreview()
		dialog.ShowInformation("保存成功", "配置已保存，需要重启传输任务才能生效。", myWindow)
	})

	// 提示标签
	hintLabel := widget.NewLabel("点击代码区域进入编辑模式")
	hintLabel.TextStyle = fyne.TextStyle{Italic: true}

	leftHeader := container.NewBorder(
		nil, nil, widget.NewLabel("配置文件 (config.toml)"), hintLabel,
	)

	leftPanel := container.NewBorder(
		leftHeader,
		saveBtn,
		nil, nil,
		switchContainer,
	)

	// ========== 右侧：控制区和日志区 ==========
	logEntry := widget.NewMultiLineEntry()
	logEntry.SetPlaceHolder("日志输出...")
	logEntry.Wrapping = fyne.TextWrapBreak
	logEntry.TextStyle = fyne.TextStyle{Monospace: true}
	logEntry.Disable()

	logWriter := NewLogWriter()
	log.SetOutput(logWriter)
	log.SetFlags(log.Ldate | log.Ltime)

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		lastText := ""
		for range ticker.C {
			text := logWriter.GetText()
			if text != lastText {
				lastText = text
				textCopy := text
				fyne.Do(func() {
					logEntry.SetText(textCopy)
				})
			}
		}
	}()

	statusLabel := widget.NewLabel("状态: 已停止")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	startBtn := widget.NewButtonWithIcon("启动任务", theme.MediaPlayIcon(), nil)
	stopBtn := widget.NewButtonWithIcon("停止任务", theme.MediaStopIcon(), nil)
	stopBtn.Disable()

	startBtn.OnTapped = func() {
		if state.IsRunning() {
			return
		}
		startBtn.Disable()

		go func() {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				log.Printf("加载配置失败: %v", err)
				fyne.Do(func() {
					startBtn.Enable()
				})
				return
			}

			hm := core.NewHistoryManager(historyPath)
			if err := hm.Load(); err != nil {
				log.Printf("警告: 加载历史记录失败: %v", err)
			}

			tm := core.NewTransferManager(hm)
			runner := core.NewRunner(cfg, tm)
			runner.Start()

			state.mu.Lock()
			state.hm = hm
			state.runner = runner
			state.running = true
			state.mu.Unlock()

			log.Println("FileTransferHX 已启动...")

			fyne.Do(func() {
				stopBtn.Enable()
				statusLabel.SetText("状态: 运行中")
			})
		}()
	}

	stopBtn.OnTapped = func() {
		if !state.IsRunning() {
			return
		}
		stopBtn.Disable()

		state.mu.Lock()
		runner := state.runner
		hm := state.hm
		state.running = false
		state.runner = nil
		state.hm = nil
		state.mu.Unlock()

		go func() {
			log.Println("正在停止...")
			if runner != nil {
				runner.Stop()
			}
			if hm != nil {
				hm.Save()
			}
			log.Println("FileTransferHX 已停止")

			fyne.Do(func() {
				startBtn.Enable()
				statusLabel.SetText("状态: 已停止")
			})
		}()
	}

	controlBar := container.NewHBox(
		statusLabel,
		widget.NewSeparator(),
		startBtn,
		stopBtn,
	)

	rightPanel := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("任务控制"),
			controlBar,
			widget.NewSeparator(),
			widget.NewLabel("运行日志"),
		),
		nil, nil, nil,
		container.NewScroll(logEntry),
	)

	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.4)

	myWindow.SetContent(split)
	myWindow.Resize(fyne.NewSize(1200, 700))

	myWindow.SetOnClosed(func() {
		state.mu.Lock()
		runner := state.runner
		hm := state.hm
		state.mu.Unlock()

		if runner != nil {
			runner.Stop()
		}
		if hm != nil {
			hm.Save()
		}
	})

	myWindow.ShowAndRun()
}

// createHighlightedLine 创建单行高亮显示
func createHighlightedLine(line string) *fyne.Container {
	trimmed := strings.TrimSpace(line)

	if trimmed == "" {
		label := canvas.NewText(" ", colorNormal)
		label.TextStyle = fyne.TextStyle{Monospace: true}
		label.TextSize = 14
		return container.NewHBox(label)
	}

	var texts []fyne.CanvasObject

	if strings.HasPrefix(trimmed, "#") {
		text := canvas.NewText(line, colorComment)
		text.TextStyle = fyne.TextStyle{Monospace: true}
		text.TextSize = 14
		return container.NewHBox(text)
	}

	if reSection.MatchString(trimmed) {
		text := canvas.NewText(line, colorSection)
		text.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		text.TextSize = 14
		return container.NewHBox(text)
	}

	if matches := reKeyVal.FindStringSubmatch(line); matches != nil {
		indent := matches[1]
		key := matches[2]
		value := matches[3]

		if indent != "" {
			t := canvas.NewText(indent, colorNormal)
			t.TextStyle = fyne.TextStyle{Monospace: true}
			t.TextSize = 14
			texts = append(texts, t)
		}

		keyText := canvas.NewText(key, colorKey)
		keyText.TextStyle = fyne.TextStyle{Monospace: true}
		keyText.TextSize = 14
		texts = append(texts, keyText)

		eqText := canvas.NewText(" = ", colorNormal)
		eqText.TextStyle = fyne.TextStyle{Monospace: true}
		eqText.TextSize = 14
		texts = append(texts, eqText)

		if commentIdx := strings.Index(value, " #"); commentIdx > 0 {
			actualValue := value[:commentIdx]
			comment := value[commentIdx:]

			valText := canvas.NewText(actualValue, getValueColor(actualValue))
			valText.TextStyle = fyne.TextStyle{Monospace: true}
			valText.TextSize = 14
			texts = append(texts, valText)

			commentText := canvas.NewText(comment, colorComment)
			commentText.TextStyle = fyne.TextStyle{Monospace: true}
			commentText.TextSize = 14
			texts = append(texts, commentText)
		} else {
			valText := canvas.NewText(value, getValueColor(value))
			valText.TextStyle = fyne.TextStyle{Monospace: true}
			valText.TextSize = 14
			texts = append(texts, valText)
		}

		return container.NewHBox(texts...)
	}

	text := canvas.NewText(line, colorNormal)
	text.TextStyle = fyne.TextStyle{Monospace: true}
	text.TextSize = 14
	return container.NewHBox(text)
}

func getValueColor(value string) color.Color {
	trimmed := strings.TrimSpace(value)

	if (strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"")) ||
		(strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'")) {
		return colorValue
	}

	if reNumber.MatchString(trimmed) {
		return colorNumber
	}

	if trimmed == "true" || trimmed == "false" {
		return colorNumber
	}

	return colorValue
}
