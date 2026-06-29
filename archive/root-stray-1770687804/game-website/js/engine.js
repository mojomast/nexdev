class GameEngine {
    constructor(canvas, context) {
        this.canvas = canvas;
        this.context = context;
        this.games = {};
        this.activeGame = null;
        this.inputManager = new InputManager();
        this.scoreManager = null;
        this.isRunning = false;
        this.isPaused = false;
        this.lastTime = 0;
        this.accumulator = 0;
        this.timeStep = 1000 / CONFIG.TARGET_FPS;
    }

    init() {
        this.inputManager.init(this.canvas);
    }

    start() {
        if (!this.isRunning) {
            this.isRunning = true;
            this.lastTime = performance.now();
            requestAnimationFrame((time) => this.gameLoop(time));
        }
    }

    stop() {
        this.isRunning = false;
    }

    pause() {
        this.isPaused = true;
    }

    resume() {
        this.isPaused = false;
        this.lastTime = performance.now();
    }

    gameLoop(currentTime) {
        if (!this.isRunning) return;

        const deltaTime = currentTime - this.lastTime;
        this.lastTime = currentTime;

        if (!this.isPaused && this.activeGame) {
            this.accumulator += deltaTime;
            while (this.accumulator >= this.timeStep) {
                this.update(this.timeStep / 1000);
                this.accumulator -= this.timeStep;
            }
            this.draw();
        }

        requestAnimationFrame((time) => this.gameLoop(time));
    }

    update(deltaTime) {
        if (this.activeGame) {
            this.activeGame.update(deltaTime);
        }
    }

    draw() {
        if (this.activeGame) {
            this.activeGame.draw(this.context);
        }
    }

    addGame(game) {
        this.games[game.id] = game;
    }

    removeGame(id) {
        delete this.games[id];
    }

    getGame(id) {
        return this.games[id];
    }

    setActiveGame(id) {
        if (this.games[id]) {
            if (this.activeGame) {
                this.activeGame.cleanup();
            }
            this.activeGame = this.games[id];
            this.activeGame.init();
            this.scoreManager = new ScoreManager(id);
            return true;
        }
        return false;
    }

    getActiveGame() {
        return this.activeGame;
    }

    getInputManager() {
        return this.inputManager;
    }

    getScoreManager() {
        return this.scoreManager;
    }

    handleGameOver() {
        if (this.scoreManager) {
            this.scoreManager.saveHighScore();
        }
        if (this.activeGame) {
            this.activeGame.cleanup();
        }
        this.activeGame = null;
        this.scoreManager = null;
    }
}
