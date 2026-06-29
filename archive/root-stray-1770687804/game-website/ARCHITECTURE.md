# Game Website Architecture

## Project Overview
A modern, responsive web-based game platform featuring a custom JavaScript/Canvas game engine with 10 playable games.

## Technology Stack
- **HTML5** - Structure and markup
- **CSS3** - Styling, animations, and responsive design
- **JavaScript (ES6+)** - Game logic and engine
- **Canvas API** - 2D rendering
- **Web Audio API** - Sound effects and music
- **LocalStorage** - High score persistence

## System Architecture

```
┌─────────────────────────────────┐
│      Frontend (HTML/CSS/JS)     │
│    - index.html (entry point)   │
│    - styles.css (styling)       │
└──────────────┬──────────────────┘
               │
               ▼
┌──────────────────────────────────┐
│       Game Engine Core          │
│  - Game Loop Manager (engine.js) │
│  - Input Handler (input.js)     │
│  - Rendering Engine             │
│  - Audio Manager                │
│  - Score/High Score System       │
│  - Game State Manager            │
└──────┬──────────┬───────────────┘
       │          │
   ┌───┴───┐  ┌─┴──────────────────┐
   │ Game 1│  │   Game 2...Game 10 │
   │(Snake)│  │  (Various Genres)  │
   └───────┘  └─────────────────────┘
               │
               ▼
┌──────────────────────────────────┐
│   Local Storage / Database     │
│  - High Scores (LocalStorage) │
│  - Game Progress              │
│  - User Settings             │
└───────────────────────────────┘
```

## Core Components

### 1. GameEngine Class
**File:** `js/engine.js`

Responsibilities:
- Manage game loop using RequestAnimationFrame
- Handle game state transitions (menu, playing, paused, game over)
- Coordinate between games, input, rendering, and audio
- Track active game and provide game lifecycle management

Methods:
- `constructor(canvas, context)` - Initialize engine with canvas
- `start()` - Start game loop
- `stop()` - Stop game loop
- `pause()` - Pause current game
- `resume()` - Resume paused game
- `addGame(game)` - Register a new game
- `removeGame(id)` - Remove a game
- `getGame(id)` - Get game by ID
- `setActiveGame(id)` - Set active game

### 2. Game (Base Class)
**File:** `js/games.js`

Responsibilities:
- Define interface all games must implement
- Provide common game functionality (score, high score)
- Handle input, update, and render lifecycle

Methods:
- `constructor(name, description)` - Initialize game with metadata
- `init()` - Set up initial game state
- `update(deltaTime)` - Update game logic
- `draw(context)` - Render game
- `handleInput(input)` - Handle player input
- `cleanup()` - Clean up resources
- `getScore()` - Get current score
- `getHighScore()` - Get high score

### 3. InputManager Class
**File:** `js/input.js`

Responsibilities:
- Capture keyboard, mouse, and touch events
- Normalize input across devices
- Maintain current input state
- Support keyboard and touch navigation

Methods:
- `constructor()` - Initialize input handlers
- `onKeyDown(key)` - Handle key press
- `onKeyUp(key)` - Handle key release
- `onMouseDown(x, y)` - Handle mouse/touch down
- `onMouseUp(x, y)` - Handle mouse/touch up
- `onTouchStart(x, y)` - Handle touch start
- `onTouchEnd(x, y)` - Handle touch end
- `getState()` - Get current input state

### 4. ScoreManager Class
**File:** `js/score.js`

Responsibilities:
- Track current game score
- Persist high scores to LocalStorage
- Load high scores from LocalStorage
- Reset scores on game restart

Methods:
- `constructor(gameId)` - Initialize with game ID
- `addScore(points)` - Add points to score
- `getScore()` - Get current score
- `getHighScore()` - Get high score
- `saveHighScore()` - Persist high score
- `loadHighScore()` - Load high score
- `resetScore()` - Reset current score

### 5. Menu System
**File:** `js/main.js`

Responsibilities:
- Display game selection menu
- Show game descriptions and high scores
- Handle menu navigation (keyboard/touch)
- Transition between menu and games

## Game Implementations

All games extend the base `Game` class and implement the required methods.

### 1. Snake
- Eat food to grow
- Avoid walls and self
- Score: food eaten

### 2. Tetris
- Rotate and place falling blocks
- Clear lines for points
- Levels increase speed

### 3. Pong
- Player vs AI
- Ball physics
- Score: balls past opponent

### 4. Breakout
- Paddle and ball
- Destroy all bricks
- Power-ups (optional)

### 5. Memory Match
- Find matching pairs
- Time limit or moves limit
- Score: pairs found

### 6. 2048
- Slide tiles to combine
- Reach 2048
- Score: tile values combined

### 7. Asteroids
- Ship controls
- Destroy asteroids
- Score: asteroids destroyed

### 8. Tic-Tac-Toe
- Player vs AI
- Win/lose detection
- Minimax AI (optional)

### 9. Whack-a-Mole
- Click/tap moles
- Time limit
- Score: moles hit

### 10. Jump (Platformer)
- Jump over obstacles
- Gravity physics
- Score: distance traveled

## File Structure

```
game-website/
├── index.html              # Main entry point
├── css/
│   └── styles.css          # All styles
├── js/
│   ├── engine.js          # Game engine core
│   ├── games.js           # All 10 game classes
│   ├── input.js           # Input handling
│   ├── score.js           # Score management
│   └── main.js            # Main application logic
├── assets/
│   ├── images/            # Game graphics
│   └── sounds/            # Audio files
└── README.md              # Documentation
```

## Configuration

Configuration via `config.js` or environment variables:

```javascript
const CONFIG = {
  SITE_TITLE: "Game Arcade",
  CANVAS_WIDTH: 800,
  CANVAS_HEIGHT: 600,
  TARGET_FPS: 60,
  HIGH_SCORE_KEY_PREFIX: "game_arena_"
};
```

## Responsive Design

- **Desktop**: Full canvas (800x600), keyboard controls
- **Tablet**: Scaled canvas, touch controls
- **Mobile**: Responsive canvas, touch controls

## Design Patterns

1. **Singleton Pattern** - GameEngine instance
2. **Factory Pattern** - Game creation
3. **Observer Pattern** - Input event handling
4. **Strategy Pattern** - Different game behaviors

## Performance Considerations

- Game loop runs at target FPS using RequestAnimationFrame
- Object pooling for game entities (if needed)
- Efficient rendering with dirty rectangle tracking
- Minimal DOM manipulation during gameplay
