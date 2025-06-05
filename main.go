package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/bitmapfont/v3"
	"github.com/hajimehoshi/ebiten/examples/resources/images"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	rkeyboard "github.com/hajimehoshi/ebiten/v2/examples/resources/images/keyboard"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// --- Constants and Globals ---

const (
	screenWidth  = 640
	screenHeight = 480
)

var (
	bgImage        *ebiten.Image
	keyboardImage  *ebiten.Image
	bulletImg      *ebiten.Image
	enemyBulletImg *ebiten.Image
	fontFace       = text.NewGoXFace(bitmapfont.Face)
	scoreFile      = "scores.json"
	scores         ScoreData
)

// --- Structs and Constructors ---

type viewport struct {
	x16 int
	y16 int
}

func (p *viewport) Move() {
	if bgImage == nil {
		return
	}
	s := bgImage.Bounds().Size()
	maxY16 := s.Y * 16
	p.y16 += s.Y / 32
	p.y16 %= maxY16
}

func (p *viewport) Position() (int, int) {
	return p.x16, p.y16
}

type Player struct {
	X, Y float64
	Size float64
}

func NewPlayer(x, y float64) *Player {
	return &Player{X: x, Y: y, Size: 32}
}

type Bullet struct {
	X, Y   float64
	SpeedY float64
	Size   float64
}

type Enemy struct {
	X, Y     float64
	Size     float64
	SpeedY   float64
	Cooldown int
	Dead     bool
}

type EnemyBullet struct {
	X, Y   float64
	SpeedX float64
	SpeedY float64
	Size   float64
}

type Game struct {
	keys          []ebiten.Key
	viewport      viewport
	player        *Player
	bullets       []*Bullet
	enemies       []*Enemy
	enemyBullets  []*EnemyBullet
	spawnCounter  int
	spawnInterval int
	elapsedFrames int
	gameState     string // "menu", "playing", "dead", "settings"
	score         int
	deathScore    int // Store score at death

	username      string
	usernameInput string

	lastGameState string // Track last state for settings

	// Settings dropdown state
	dropdownOpen   bool
	selectedScreen int
	customWidth    int
	customHeight   int
	customInput    bool
	customInputStr string
}

type ScoreData struct {
	HighScores map[string]int `json:"high_scores"`
}

// --- Asset Initialization ---

func init() {
	rand.Seed(time.Now().UnixNano())

	// Load keyboard image
	img, _, err := image.Decode(bytes.NewReader(rkeyboard.Keyboard_png))
	if err != nil {
		log.Fatal(err)
	}
	keyboardImage = ebiten.NewImageFromImage(img)

	// Load background image
	imgBG, _, err := image.Decode(bytes.NewReader(images.Tile_png))
	if err != nil {
		log.Fatal(err)
	}
	bgImage = ebiten.NewImageFromImage(imgBG)

	bulletImg = ebiten.NewImage(6, 6)
	bulletImg.Fill(color.RGBA{255, 255, 0, 255})

	enemyBulletImg = ebiten.NewImage(6, 6)
	enemyBulletImg.Fill(color.RGBA{0, 255, 255, 255})
}

// --- Utility Functions ---

func rectsOverlap(x1, y1, s1, x2, y2, s2 float64) bool {
	return x1 < x2+s2 && x2 < x1+s1 && y1 < y2+s2 && y2 < y1+s1
}

func loadScores() {
	scores.HighScores = make(map[string]int)
	f, err := os.Open(scoreFile)
	if err != nil {
		return // No file yet
	}
	defer f.Close()
	json.NewDecoder(f).Decode(&scores)
}

func saveScores() {
	f, err := os.Create(scoreFile)
	if err != nil {
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(scores)
}

func getTopScores(n int) [][2]string {
	type pair struct {
		Name  string
		Score int
	}
	var pairs []pair
	for name, score := range scores.HighScores {
		pairs = append(pairs, pair{name, score})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Score > pairs[j].Score
	})
	top := [][2]string{}
	for i := 0; i < n && i < len(pairs); i++ {
		top = append(top, [2]string{pairs[i].Name, fmt.Sprintf("%d", pairs[i].Score)})
	}
	return top
}

// --- Game Methods ---

func (g *Game) Update() error {
	g.keys = inpututil.AppendPressedKeys(g.keys[:0])

	// --- Settings Page Logic ---
	if g.gameState == "settings" {
		centerX := float64(screenWidth) / 2
		cardH := 300.0
		cardY := float64(screenHeight)/2 - cardH/2
		btnW, btnH := 120.0, 40.0
		btnX := centerX - btnW/2
		btnY := cardY + cardH - btnH - 24

		// Dropdown area
		ddX, ddY := centerX-100.0, cardY+100.0
		ddW, ddH := 200.0, 32.0
		screenSizes := []struct {
			Label string
			W, H  int
		}{
			{"640 x 480", 640, 480},
			{"800 x 600", 800, 600},
			{"1024 x 768", 1024, 768},
			{"Custom...", 0, 0},
		}

		if g.customInput {
			// Handle custom input (format: width,height)
			for _, r := range ebiten.AppendInputChars(nil) {
				if (r >= '0' && r <= '9') || r == ',' {
					g.customInputStr += string(r)
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.customInputStr) > 0 {
				g.customInputStr = g.customInputStr[:len(g.customInputStr)-1]
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				parts := strings.Split(g.customInputStr, ",")
				if len(parts) == 2 {
					w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
					h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
					if err1 == nil && err2 == nil && w > 100 && h > 100 {
						g.customWidth = w
						g.customHeight = h
						ebiten.SetWindowSize(w*2, h*2)
						g.selectedScreen = 3
						g.customInput = false
						g.customInputStr = ""
					}
				}
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
				g.customInput = false
				g.customInputStr = ""
			}
			return nil
		}

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			xf, yf := float64(x), float64(y)
			// Dropdown click
			if xf >= ddX && xf <= ddX+ddW && yf >= ddY && yf <= ddY+ddH {
				g.dropdownOpen = !g.dropdownOpen
			} else if g.dropdownOpen {
				// Check if clicked on an option
				for i := range screenSizes {
					optY := ddY + ddH + float64(i)*ddH
					if xf >= ddX && xf <= ddX+ddW && yf >= optY && yf <= optY+ddH {
						if i == 3 {
							g.customInput = true
							g.customInputStr = ""
						} else {
							g.selectedScreen = i
							ebiten.SetWindowSize(screenSizes[i].W*2, screenSizes[i].H*2)
						}
						g.dropdownOpen = false
						break
					}
				}
			}
			// Back button
			if xf >= btnX && xf <= btnX+btnW && yf >= btnY && yf <= btnY+btnH {
				if g.lastGameState != "" {
					g.gameState = g.lastGameState
				} else {
					g.gameState = "menu"
				}
			}
		}
		return nil
	}

	// --- Start Menu Logic ---
	if g.gameState == "menu" {
		// Handle username input
		for _, r := range ebiten.AppendInputChars(nil) {
			if len(g.usernameInput) < 12 && (r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				g.usernameInput += string(r)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.usernameInput) > 0 {
			g.usernameInput = g.usernameInput[:len(g.usernameInput)-1]
		}
		// Enter to confirm username and start
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && len(g.usernameInput) > 0 {
			g.username = g.usernameInput
			g.Reset()
			g.gameState = "playing"
		}

		// --- Settings Button Click Logic ---
		centerX := float64(screenWidth) / 2
		cardH := 420.0
		cardY := float64(screenHeight)/2 - cardH/2
		btnW, btnH := 120.0, 40.0
		btnX := centerX - btnW/2
		btnY := cardY + cardH - btnH - 24
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			xf, yf := float64(x), float64(y)
			if xf >= btnX && xf <= btnX+btnW && yf >= btnY && yf <= btnY+btnH {
				g.lastGameState = "menu" // <--- Track last state
				g.gameState = "settings"
			}
		}

		return nil
	}

	// --- Death Screen Logic ---
	if g.gameState == "dead" {
		centerX := float64(screenWidth) / 2
		cardH := 300.0
		cardY := float64(screenHeight)/2 - cardH/2
		btnW, btnH := 120.0, 40.0
		btnX := centerX - btnW/2

		// Button Y positions
		menuBtnY := cardY + cardH - btnH*3 - 24 - 16 // Top button: Main Menu
		playAgainBtnY := menuBtnY + btnH + 16        // Middle button: Play Again
		settingsBtnY := playAgainBtnY + btnH + 16    // Bottom button: Settings

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := ebiten.CursorPosition()
			xf, yf := float64(x), float64(y)
			// Main Menu button
			if xf >= btnX && xf <= btnX+btnW && yf >= menuBtnY && yf <= menuBtnY+btnH {
				g.Reset()
				g.gameState = "menu"
			}
			// Play Again button
			if xf >= btnX && xf <= btnX+btnW && yf >= playAgainBtnY && yf <= playAgainBtnY+btnH {
				g.Reset()
				g.gameState = "playing"
			}
			// Settings button
			if xf >= btnX && xf <= btnX+btnW && yf >= settingsBtnY && yf <= settingsBtnY+btnH {
				g.lastGameState = "dead" // <--- Track last state
				g.gameState = "settings"
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.Reset()
			g.gameState = "playing"
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.Reset()
			g.gameState = "menu"
		}
		return nil
	}

	g.viewport.Move()
	g.elapsedFrames++ // Track time

	// Player movement
	const speed = 4.0
	if g.player != nil {
		if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
			g.player.Y -= speed
		}
		if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
			g.player.Y += speed
		}
		if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
			g.player.X -= speed
		}
		if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
			g.player.X += speed
		}
		// Clamp to screen
		if g.player.X < 0 {
			g.player.X = 0
		}
		if g.player.Y < 0 {
			g.player.Y = 0
		}
		if g.player.X > float64(screenWidth)-g.player.Size {
			g.player.X = float64(screenWidth) - g.player.Size
		}
		if g.player.Y > float64(screenHeight)-g.player.Size {
			g.player.Y = float64(screenHeight) - g.player.Size
		}
	}

	// Shooting
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && g.player != nil {
		bullet := &Bullet{
			X:      g.player.X + g.player.Size/2 - 3,
			Y:      g.player.Y + g.player.Size,
			SpeedY: 8,
			Size:   6,
		}
		g.bullets = append(g.bullets, bullet)
	}

	// Gradually decrease spawnInterval, but not below a minimum (e.g., 10)
	if g.elapsedFrames%120 == 0 && g.spawnInterval > 10 {
		g.spawnInterval -= 5
		if g.spawnInterval < 10 {
			g.spawnInterval = 10
		}
	}

	// Enemy spawning
	g.spawnCounter++
	if g.spawnCounter >= g.spawnInterval {
		g.spawnCounter = 0
		numEnemies := 1 + (90-g.spawnInterval)/20
		for i := 0; i < numEnemies; i++ {
			enemy := &Enemy{
				X:        float64(32 + rand.Intn(screenWidth-64)),
				Y:        float64(screenHeight),
				Size:     32,
				SpeedY:   -2,
				Cooldown: 30 + rand.Intn(60),
			}
			g.enemies = append(g.enemies, enemy)
		}
	}

	// Enemy movement and shooting
	var movedEnemies []*Enemy
	for _, e := range g.enemies {
		e.Y += e.SpeedY
		if e.Y+e.Size < 0 {
			continue
		}
		if g.player != nil {
			e.Cooldown--
			if e.Cooldown <= 0 {
				dx := (g.player.X + g.player.Size/2) - (e.X + e.Size/2)
				dy := (g.player.Y + g.player.Size/2) - (e.Y + e.Size/2)
				dist := dx*dx + dy*dy
				if dist > 0 {
					length := math.Sqrt(dist)
					speed := 5.0
					eb := &EnemyBullet{
						X:      e.X + e.Size/2 - 3,
						Y:      e.Y + e.Size/2 - 3,
						SpeedX: dx / length * speed,
						SpeedY: dy / length * speed,
						Size:   6,
					}
					g.enemyBullets = append(g.enemyBullets, eb)
					e.Cooldown = 60 + rand.Intn(60)
				}
			}
		}
		movedEnemies = append(movedEnemies, e)
	}
	g.enemies = movedEnemies

	// Enemy bullets movement
	var activeEnemyBullets []*EnemyBullet
	for _, eb := range g.enemyBullets {
		eb.X += eb.SpeedX
		eb.Y += eb.SpeedY
		if eb.X+eb.Size > 0 && eb.X < float64(screenWidth) && eb.Y+eb.Size > 0 && eb.Y < float64(screenHeight) {
			activeEnemyBullets = append(activeEnemyBullets, eb)
		}
	}
	g.enemyBullets = activeEnemyBullets

	// Player bullets movement
	var movedBullets []*Bullet
	for _, b := range g.bullets {
		b.Y += b.SpeedY
		if b.Y+b.Size > 0 {
			movedBullets = append(movedBullets, b)
		}
	}
	g.bullets = movedBullets

	// Bullet vs Enemy collision
	var remainingBullets []*Bullet
	for _, b := range g.bullets {
		hit := false
		for _, e := range g.enemies {
			if !e.Dead && rectsOverlap(b.X, b.Y, b.Size, e.X, e.Y, e.Size) {
				e.Dead = true
				hit = true
				g.score++
				break
			}
		}
		if !hit {
			remainingBullets = append(remainingBullets, b)
		}
	}
	// Remove dead enemies
	var survivedEnemies []*Enemy
	for _, e := range g.enemies {
		if !e.Dead {
			survivedEnemies = append(survivedEnemies, e)
		}
	}
	g.enemies = survivedEnemies
	g.bullets = remainingBullets

	// Enemy bullet vs Player collision
	if g.player != nil {
		var activeEnemyBullets []*EnemyBullet
		playerHit := false
		for _, eb := range g.enemyBullets {
			if rectsOverlap(eb.X, eb.Y, eb.Size, g.player.X, g.player.Y, g.player.Size) {
				playerHit = true
				continue
			}
			activeEnemyBullets = append(activeEnemyBullets, eb)
		}
		g.enemyBullets = activeEnemyBullets
		if playerHit {
			g.deathScore = g.score
			// --- Save high score if it's a new record ---
			if g.username != "" && g.score > scores.HighScores[g.username] {
				scores.HighScores[g.username] = g.score
				saveScores()
			}
			g.gameState = "dead"
			return nil
		}
	}

	// Player vs Enemy collision
	if g.player != nil {
		for _, e := range g.enemies {
			if rectsOverlap(g.player.X, g.player.Y, g.player.Size, e.X, e.Y, e.Size) {
				g.deathScore = g.score
				// --- Save high score if it's a new record ---
				if g.username != "" && g.score > scores.HighScores[g.username] {
					scores.HighScores[g.username] = g.score
					saveScores()
				}
				g.gameState = "dead"
				break
			}
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.gameState == "menu" {
		screen.Fill(color.RGBA{0, 0, 0, 255})

		// Card background
		cardW, cardH := 400.0, 420.0
		cardX := float64(screenWidth)/2 - cardW/2
		cardY := float64(screenHeight)/2 - cardH/2
		cardImg := ebiten.NewImage(int(cardW), int(cardH))
		cardImg.Fill(color.RGBA{30, 30, 40, 220})
		cardOp := &ebiten.DrawImageOptions{}
		cardOp.GeoM.Translate(cardX, cardY)
		screen.DrawImage(cardImg, cardOp)

		// Centered text positions
		centerX := float64(screenWidth) / 2
		y := cardY + 36

		title := "2D-GO"
		instr := "Enter Username:"
		userInput := g.usernameInput
		if len(userInput) == 0 {
			userInput = "_"
		}
		startMsg := "Press ENTER to Start"
		if len(g.usernameInput) == 0 {
			startMsg = "Type your name to enable Start"
		}
		highScoreMsg := ""
		if g.usernameInput != "" {
			highScore := scores.HighScores[g.usernameInput]
			highScoreMsg = fmt.Sprintf("High Score: %d", highScore)
		}

		// Draw Title
		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(centerX-float64(len(title))*4, y)
		text.Draw(screen, title, fontFace, textOp)
		y += 48

		// Draw instruction
		textOpInstr := &text.DrawOptions{}
		textOpInstr.GeoM.Translate(centerX-float64(len(instr))*4, y)
		text.Draw(screen, instr, fontFace, textOpInstr)
		y += 36

		// Draw username input
		textOpInput := &text.DrawOptions{}
		textOpInput.GeoM.Translate(centerX-float64(len(userInput))*4, y)
		text.Draw(screen, userInput, fontFace, textOpInput)
		y += 36

		// Draw start message
		textOpStart := &text.DrawOptions{}
		textOpStart.GeoM.Translate(centerX-float64(len(startMsg))*4, y)
		text.Draw(screen, startMsg, fontFace, textOpStart)
		y += 36

		// Draw high score if available
		if highScoreMsg != "" {
			textOpHS := &text.DrawOptions{}
			textOpHS.GeoM.Translate(centerX-float64(len(highScoreMsg))*4, y)
			text.Draw(screen, highScoreMsg, fontFace, textOpHS)
			y += 36
		}

		// Draw leaderboard title
		leaderboardTitle := "Leaderboard (Top 10)"
		textOpLB := &text.DrawOptions{}
		textOpLB.GeoM.Translate(centerX-float64(len(leaderboardTitle))*4, y)
		text.Draw(screen, leaderboardTitle, fontFace, textOpLB)
		y += 32

		// Draw leaderboard entries (centered)
		topScores := getTopScores(10)
		for i, entry := range topScores {
			name := entry[0]
			score := entry[1]
			line := fmt.Sprintf("%2d. %-12s %6s", i+1, name, score)
			lineWidth := float64(len(line)) * 8 // 8px per character (monospace)
			textOpEntry := &text.DrawOptions{}
			textOpEntry.GeoM.Translate(centerX-lineWidth/2, y+float64(i*24))
			text.Draw(screen, line, fontFace, textOpEntry)
		}

		// --- Settings Button ---
		btnW, btnH := 120.0, 40.0
		btnX := centerX - btnW/2
		btnY := cardY + cardH - btnH - 24
		btnImg := ebiten.NewImage(int(btnW), int(btnH))
		btnImg.Fill(color.RGBA{60, 60, 120, 200})
		btnOp := &ebiten.DrawImageOptions{}
		btnOp.GeoM.Translate(btnX, btnY)
		screen.DrawImage(btnImg, btnOp)

		btnText := "Settings"
		btnTextWidth := float64(len(btnText)) * 8
		btnTextOp := &text.DrawOptions{}
		btnTextOp.GeoM.Translate(centerX-btnTextWidth/2, btnY+10)
		text.Draw(screen, btnText, fontFace, btnTextOp)

		return
	}

	// --- Settings Page ---
	if g.gameState == "settings" {
		screen.Fill(color.RGBA{20, 20, 40, 255})

		centerX := float64(screenWidth) / 2
		cardW, cardH := 400.0, 300.0
		cardX := centerX - cardW/2
		cardY := float64(screenHeight)/2 - cardH/2

		cardImg := ebiten.NewImage(int(cardW), int(cardH))
		cardImg.Fill(color.RGBA{40, 40, 60, 220})
		cardOp := &ebiten.DrawImageOptions{}
		cardOp.GeoM.Translate(cardX, cardY)
		screen.DrawImage(cardImg, cardOp)

		title := "Settings"
		titleWidth := float64(len(title)) * 8
		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(centerX-titleWidth/2, cardY+36)
		text.Draw(screen, title, fontFace, textOp)

		// --- Dropdown for screen size ---
		ddX, ddY := centerX-100.0, cardY+100.0
		ddW, ddH := 200.0, 32.0
		screenSizes := []string{"640 x 480", "800 x 600", "1024 x 768", "Custom..."}

		// Draw dropdown box
		ddImg := ebiten.NewImage(int(ddW), int(ddH))
		ddImg.Fill(color.RGBA{80, 80, 120, 255})
		ddOp := &ebiten.DrawImageOptions{}
		ddOp.GeoM.Translate(ddX, ddY)
		screen.DrawImage(ddImg, ddOp)

		// Draw selected option
		selText := screenSizes[g.selectedScreen]
		if g.selectedScreen == 3 && g.customWidth > 0 && g.customHeight > 0 {
			selText = fmt.Sprintf("Custom: %dx%d", g.customWidth, g.customHeight)
		}
		selTextOp := &text.DrawOptions{}
		selTextOp.GeoM.Translate(ddX+12, ddY+8)
		text.Draw(screen, selText, fontFace, selTextOp)

		// Draw dropdown arrow
		arrow := "▼"
		arrowOp := &text.DrawOptions{}
		arrowOp.GeoM.Translate(ddX+ddW-24, ddY+8)
		text.Draw(screen, arrow, fontFace, arrowOp)

		// Draw options if open
		if g.dropdownOpen {
			for i, opt := range screenSizes {
				optImg := ebiten.NewImage(int(ddW), int(ddH))
				optImg.Fill(color.RGBA{60, 60, 100, 230})
				optOp := &ebiten.DrawImageOptions{}
				optOp.GeoM.Translate(ddX, ddY+ddH+float64(i)*ddH)
				screen.DrawImage(optImg, optOp)

				optTextOp := &text.DrawOptions{}
				optTextOp.GeoM.Translate(ddX+12, ddY+ddH+float64(i)*ddH+8)
				text.Draw(screen, opt, fontFace, optTextOp)
			}
		}

		// --- Custom input dialog ---
		if g.customInput {
			dialogW, dialogH := 260.0, 80.0
			dialogX := centerX - dialogW/2
			dialogY := cardY + 160
			dialogImg := ebiten.NewImage(int(dialogW), int(dialogH))
			dialogImg.Fill(color.RGBA{30, 30, 60, 240})
			dialogOp := &ebiten.DrawImageOptions{}
			dialogOp.GeoM.Translate(dialogX, dialogY)
			screen.DrawImage(dialogImg, dialogOp)

			prompt := "Enter width,height (e.g. 900,700):"
			promptOp := &text.DrawOptions{}
			promptOp.GeoM.Translate(dialogX+12, dialogY+16)
			text.Draw(screen, prompt, fontFace, promptOp)

			inputOp := &text.DrawOptions{}
			inputOp.GeoM.Translate(dialogX+12, dialogY+40)
			text.Draw(screen, g.customInputStr, fontFace, inputOp)
		}

		// --- Back button ---
		btnW, btnH := 120.0, 40.0
		btnX := centerX - btnW/2
		btnY := cardY + cardH - btnH - 24
		btnImg := ebiten.NewImage(int(btnW), int(btnH))
		btnImg.Fill(color.RGBA{80, 80, 80, 200})
		btnOp := &ebiten.DrawImageOptions{}
		btnOp.GeoM.Translate(btnX, btnY)
		screen.DrawImage(btnImg, btnOp)

		btnText := "Back"
		btnTextWidth := float64(len(btnText)) * 8
		btnTextOp := &text.DrawOptions{}
		btnTextOp.GeoM.Translate(centerX-btnTextWidth/2, btnY+10)
		text.Draw(screen, btnText, fontFace, btnTextOp)

		return
	}

	// --- Death Screen ---
	if g.gameState == "dead" {
		screen.Fill(color.RGBA{20, 0, 0, 255})

		centerX := float64(screenWidth) / 2
		cardW, cardH := 400.0, 300.0
		cardX := centerX - cardW/2
		cardY := float64(screenHeight)/2 - cardH/2

		cardImg := ebiten.NewImage(int(cardW), int(cardH))
		cardImg.Fill(color.RGBA{60, 20, 20, 220})
		cardOp := &ebiten.DrawImageOptions{}
		cardOp.GeoM.Translate(cardX, cardY)
		screen.DrawImage(cardImg, cardOp)

		title := "Game Over"
		scoreMsg := fmt.Sprintf("Score: %d", g.deathScore)
		highScoreMsg := ""
		if g.username != "" {
			highScore := scores.HighScores[g.username]
			highScoreMsg = fmt.Sprintf("High Score: %d", highScore)
		}

		y := cardY + 36
		textOp := &text.DrawOptions{}
		textOp.GeoM.Translate(centerX-float64(len(title))*4, y)
		text.Draw(screen, title, fontFace, textOp)
		y += 48

		textOpScore := &text.DrawOptions{}
		textOpScore.GeoM.Translate(centerX-float64(len(scoreMsg))*4, y)
		text.Draw(screen, scoreMsg, fontFace, textOpScore)
		y += 36

		if highScoreMsg != "" {
			textOpHS := &text.DrawOptions{}
			textOpHS.GeoM.Translate(centerX-float64(len(highScoreMsg))*4, y)
			text.Draw(screen, highScoreMsg, fontFace, textOpHS)
			y += 36
		}

		// Button Y positions
		btnW, btnH := 120.0, 40.0
		btnX := centerX - btnW/2
		menuBtnY := cardY + cardH - btnH*3 - 24 - 16
		playAgainBtnY := menuBtnY + btnH + 16
		settingsBtnY := playAgainBtnY + btnH + 16

		// Main Menu button
		menuBtnImg := ebiten.NewImage(int(btnW), int(btnH))
		menuBtnImg.Fill(color.RGBA{80, 80, 80, 200})
		menuBtnOp := &ebiten.DrawImageOptions{}
		menuBtnOp.GeoM.Translate(btnX, menuBtnY)
		screen.DrawImage(menuBtnImg, menuBtnOp)
		menuBtnText := "Main Menu"
		menuBtnTextWidth := float64(len(menuBtnText)) * 8
		menuBtnTextOp := &text.DrawOptions{}
		menuBtnTextOp.GeoM.Translate(centerX-menuBtnTextWidth/2, menuBtnY+10)
		text.Draw(screen, menuBtnText, fontFace, menuBtnTextOp)

		// Play Again button
		playAgainBtnImg := ebiten.NewImage(int(btnW), int(btnH))
		playAgainBtnImg.Fill(color.RGBA{60, 60, 120, 200})
		playAgainBtnOp := &ebiten.DrawImageOptions{}
		playAgainBtnOp.GeoM.Translate(btnX, playAgainBtnY)
		screen.DrawImage(playAgainBtnImg, playAgainBtnOp)
		playAgainBtnText := "Play Again"
		playAgainBtnTextWidth := float64(len(playAgainBtnText)) * 8
		playAgainBtnTextOp := &text.DrawOptions{}
		playAgainBtnTextOp.GeoM.Translate(centerX-playAgainBtnTextWidth/2, playAgainBtnY+10)
		text.Draw(screen, playAgainBtnText, fontFace, playAgainBtnTextOp)

		// Settings button
		settingsBtnImg := ebiten.NewImage(int(btnW), int(btnH))
		settingsBtnImg.Fill(color.RGBA{60, 60, 120, 200})
		settingsBtnOp := &ebiten.DrawImageOptions{}
		settingsBtnOp.GeoM.Translate(btnX, settingsBtnY)
		screen.DrawImage(settingsBtnImg, settingsBtnOp)
		settingsBtnText := "Settings"
		settingsBtnTextWidth := float64(len(settingsBtnText)) * 8
		settingsBtnTextOp := &text.DrawOptions{}
		settingsBtnTextOp.GeoM.Translate(centerX-settingsBtnTextWidth/2, settingsBtnY+10)
		text.Draw(screen, settingsBtnText, fontFace, settingsBtnTextOp)

		return
	}

	// Draw background
	if bgImage != nil {
		x16, y16 := g.viewport.Position()
		offsetX, offsetY := float64(-x16)/16, float64(-y16)/16
		const repeat = 3
		w, h := bgImage.Bounds().Dx(), bgImage.Bounds().Dy()
		for j := 0; j < repeat; j++ {
			for i := 0; i < repeat; i++ {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(w*i), float64(h*j))
				op.GeoM.Translate(offsetX, offsetY)
				screen.DrawImage(bgImage, op)
			}
		}
	}

	// Draw player
	if g.player != nil {
		playerRect := ebiten.NewImage(int(g.player.Size), int(g.player.Size))
		playerRect.Fill(color.RGBA{255, 0, 0, 255})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(g.player.X, g.player.Y)
		screen.DrawImage(playerRect, op)
	}

	// Draw bullets
	for _, b := range g.bullets {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(b.X, b.Y)
		screen.DrawImage(bulletImg, op)
	}

	// Draw enemies
	for _, e := range g.enemies {
		enemyRect := ebiten.NewImage(int(e.Size), int(e.Size))
		enemyRect.Fill(color.RGBA{0, 0, 255, 255})
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(e.X, e.Y)
		screen.DrawImage(enemyRect, op)
	}

	// Draw enemy bullets
	for _, eb := range g.enemyBullets {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(eb.X, eb.Y)
		screen.DrawImage(enemyBulletImg, op)
	}

	// Draw keyboard input info
	var keyStrs []string
	var keyNames []string
	for _, k := range g.keys {
		keyStrs = append(keyStrs, k.String())
		if name := ebiten.KeyName(k); name != "" {
			keyNames = append(keyNames, name)
		}
	}
	textOp := &text.DrawOptions{}
	textOp.LineSpacing = fontFace.Metrics().HLineGap + fontFace.Metrics().HAscent + fontFace.Metrics().HDescent
	text.Draw(screen, strings.Join(keyStrs, ", ")+"\n"+strings.Join(keyNames, ", "), fontFace, textOp)

	// Draw score
	scoreStr := fmt.Sprintf("Score: %d", g.score)
	textOpScore := &text.DrawOptions{}
	textWidth := float64(len(scoreStr)) * 8
	textHeight := 20.0
	scoreX := float64(screenWidth) - textWidth - 20
	scoreY := 10.0

	rectImg := ebiten.NewImage(int(textWidth+16), int(textHeight))
	rectImg.Fill(color.RGBA{0, 0, 0, 128})
	rectOp := &ebiten.DrawImageOptions{}
	rectOp.GeoM.Translate(scoreX-8, scoreY-2)
	screen.DrawImage(rectImg, rectOp)

	textOpScore.GeoM.Translate(scoreX, scoreY)
	text.Draw(screen, scoreStr, fontFace, textOpScore)

	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %0.2f", ebiten.ActualTPS()))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *Game) Reset() {
	g.player = NewPlayer(float64(screenWidth/2), float64(screenHeight/2))
	g.bullets = []*Bullet{}
	g.enemies = []*Enemy{}
	g.enemyBullets = []*EnemyBullet{}
	g.spawnCounter = 0
	g.spawnInterval = 90
	g.elapsedFrames = 0
	g.score = 0
	// Don't reset username or usernameInput here!
}

// --- Main ---

func main() {
	loadScores()
	ebiten.SetWindowSize(screenWidth*2, screenHeight*2)
	ebiten.SetWindowTitle("Keyboard + Scrolling Background (Ebitengine Demo)")
	game := &Game{
		gameState:      "menu",
		player:         NewPlayer(float64(screenWidth/2), float64(screenHeight/2)),
		enemies:        []*Enemy{},
		enemyBullets:   []*EnemyBullet{},
		spawnInterval:  90,
		dropdownOpen:   false,
		selectedScreen: 0, // 0: 640x480, 1: 800x600, 2: 1024x768
	}
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
