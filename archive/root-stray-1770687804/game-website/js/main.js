function initializeArcade() {
    const menuContainer = document.getElementById("menu-container");
    const gameContainer = document.getElementById("game-container");
    const gameList = document.getElementById("game-list");
    const gameTitle = document.getElementById("game-title");
    const canvas = document.getElementById("game-canvas");
    const scoreEl = document.getElementById("score");
    const highScoreEl = document.getElementById("highscore");
    const pauseOverlay = document.getElementById("pause-overlay");
    const gameOverOverlay = document.getElementById("gameover-overlay");
    const finalScore = document.getElementById("final-score");

    const backButton = document.getElementById("back-button");
    const pauseButton = document.getElementById("pause-button");
    const restartButton = document.getElementById("restart-button");
    const resumeButton = document.getElementById("resume-button");
    const quitButton = document.getElementById("quit-button");
    const playAgainButton = document.getElementById("playagain-button");
    const gameOverQuitButton = document.getElementById("gameover-quit-button");

    document.title = CONFIG.SITE_TITLE;
    canvas.width = CONFIG.CANVAS_WIDTH;
    canvas.height = CONFIG.CANVAS_HEIGHT;

    const context = canvas.getContext("2d");
    const engine = new GameEngine(canvas, context);
    engine.init();

    let currentGameId = null;
    let gameOverShown = false;

    function updateHud() {
        const scoreManager = engine.getScoreManager();
        scoreEl.textContent = scoreManager ? scoreManager.getScore() : "0";
        highScoreEl.textContent = scoreManager ? scoreManager.getHighScore() : "0";
    }

    function returnToMenu() {
        engine.stop();
        if (engine.getActiveGame()) {
            engine.getActiveGame().cleanup();
        }
        currentGameId = null;
        gameOverShown = false;

        menuContainer.classList.remove("hidden");
        gameContainer.classList.add("hidden");
        pauseOverlay.classList.add("hidden");
        gameOverOverlay.classList.add("hidden");

        renderGameCards();
    }

    function showGameOver(reason) {
        if (gameOverShown) {
            return;
        }
        gameOverShown = true;

        engine.pause();

        const scoreManager = engine.getScoreManager();
        const score = scoreManager ? scoreManager.getScore() : 0;
        if (scoreManager) {
            scoreManager.saveHighScore();
        }

        finalScore.textContent = `Final Score: ${score}${reason ? ` (${reason})` : ""}`;
        updateHud();
        gameOverOverlay.classList.remove("hidden");
    }

    function startGame(gameId) {
        const game = engine.getGame(gameId);
        if (!game) {
            return;
        }

        const success = engine.setActiveGame(gameId);
        if (!success) {
            return;
        }

        currentGameId = gameId;
        gameOverShown = false;

        engine.getActiveGame().onGameOver = showGameOver;
        gameTitle.textContent = game.name;
        pauseButton.textContent = "Pause";

        menuContainer.classList.add("hidden");
        gameContainer.classList.remove("hidden");
        pauseOverlay.classList.add("hidden");
        gameOverOverlay.classList.add("hidden");

        updateHud();
        engine.resume();
        engine.start();
    }

    function renderGameCards() {
        gameList.innerHTML = "";

        for (const def of GAME_DEFINITIONS) {
            const scoreManager = new ScoreManager(def.id);
            const card = document.createElement("div");
            card.className = "game-card";
            card.innerHTML = `
                <h3>${def.name}</h3>
                <p>${def.description}</p>
                <div class="highscore">Best: ${scoreManager.getHighScore()}</div>
            `;
            card.addEventListener("click", () => startGame(def.id));
            gameList.appendChild(card);
        }
    }

    for (const def of GAME_DEFINITIONS) {
        engine.addGame(new ArcadeGame(def, engine));
    }

    backButton.addEventListener("click", returnToMenu);

    pauseButton.addEventListener("click", () => {
        if (!currentGameId) {
            return;
        }
        if (engine.isPaused) {
            engine.resume();
            pauseOverlay.classList.add("hidden");
            pauseButton.textContent = "Pause";
        } else {
            engine.pause();
            pauseOverlay.classList.remove("hidden");
            pauseButton.textContent = "Resume";
        }
    });

    restartButton.addEventListener("click", () => {
        if (!currentGameId) {
            return;
        }
        startGame(currentGameId);
    });

    resumeButton.addEventListener("click", () => {
        engine.resume();
        pauseOverlay.classList.add("hidden");
        pauseButton.textContent = "Pause";
    });

    quitButton.addEventListener("click", returnToMenu);
    playAgainButton.addEventListener("click", () => {
        if (currentGameId) {
            startGame(currentGameId);
        }
    });
    gameOverQuitButton.addEventListener("click", returnToMenu);

    window.addEventListener("keydown", (event) => {
        if (event.code === "Escape" && currentGameId && !gameOverOverlay.classList.contains("hidden")) {
            return;
        }
        if (event.code === "Escape" && currentGameId) {
            pauseButton.click();
        }
    });

    function monitorGameState() {
        const activeGame = engine.getActiveGame();
        if (activeGame && activeGame.isGameOver) {
            showGameOver(activeGame.gameOverReason);
        }
        updateHud();
        requestAnimationFrame(monitorGameState);
    }

    renderGameCards();
    monitorGameState();
}

window.addEventListener("DOMContentLoaded", initializeArcade);
