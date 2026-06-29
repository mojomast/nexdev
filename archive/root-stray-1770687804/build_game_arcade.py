#!/usr/bin/env python3
"""
Complete MCP-Only Workflow: Build 10-Game Arcade Website
Uses ONLY Geoffrey MCP tools - no direct file operations
"""
import json
import subprocess
import sys
import time

class MCPClient:
    def __init__(self, geoffrussy_bin, project_path):
        self.project_path = project_path
        self.process = subprocess.Popen(
            [geoffrussy_bin, "mcp-server", "--project-path", project_path, "--debug"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1
        )
        self.request_id = 0
        self.log_file = open("MCP_COMPLETE_LOG.md", "w")
        self.log("# Complete MCP Workflow Log: 10-Game Arcade\n")
        self.log(f"**Project:** {project_path}")
        self.log(f"**Date:** 2026-01-30")
        self.log(f"**Method:** Pure MCP JSON-RPC calls\n")
        self.log("---\n")

    def log(self, message):
        """Log to both console and file"""
        print(message)
        self.log_file.write(message + "\n")
        self.log_file.flush()

    def send_request(self, method, params=None):
        self.request_id += 1
        request = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method
        }
        if params is not None:
            request["params"] = params

        # Log the request
        self.log(f"\n## MCP Request #{self.request_id}: {method}")
        self.log(f"```json\n{json.dumps(request, indent=2)}\n```")

        self.process.stdin.write(json.dumps(request) + "\n")
        self.process.stdin.flush()

        # Read response
        response_line = self.process.stdout.readline()
        if response_line:
            response = json.loads(response_line)
            self.log(f"\n**Response:**")
            self.log(f"```json\n{json.dumps(response, indent=2)}\n```")
            return response
        return None

    def send_notification(self, method, params=None):
        request = {
            "jsonrpc": "2.0",
            "method": method
        }
        if params is not None:
            request["params"] = params

        self.process.stdin.write(json.dumps(request) + "\n")
        self.process.stdin.flush()

    def call_tool(self, tool_name, arguments):
        """Call an MCP tool"""
        return self.send_request("tools/call", {
            "name": tool_name,
            "arguments": arguments
        })

    def close(self):
        self.log_file.close()
        self.process.stdin.close()
        self.process.wait()


def main():
    project_path = "/home/mojo/projects/geoffrussy/geoffrussy/game-arcade-mcp"
    geoffrussy_bin = "/home/mojo/projects/geoffrussy/geoffrussy/bin/geoffrussy"

    client = MCPClient(geoffrussy_bin, project_path)

    try:
        # Step 1: Initialize MCP connection
        client.log("\n" + "=" * 80)
        client.log("STEP 1: Initialize MCP Connection")
        client.log("=" * 80)

        result = client.send_request("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "autonomous-builder", "version": "1.0"}
        })
        client.send_notification("initialized")

        # Step 2: Start Interview
        client.log("\n" + "=" * 80)
        client.log("STEP 2: Start Project Interview")
        client.log("=" * 80)

        result = client.call_tool("run_interview", {
            "projectPath": project_path,
            "model": "glm-4.7"
        })

        # Interview answers for 10-game arcade website (mapped to actual question IDs)
        interview_answers = {
            # Phase 1: Project Essence
            "pe_1": "An interactive web-based arcade featuring 10 classic games (Snake, Pong, Breakout, Tetris, Space Invaders, Memory Match, 2048, Tic Tac Toe, Simon Says, Flappy Bird) with modern design, responsive gameplay, and high score tracking.",
            "pe_2": "Players of all ages who enjoy classic arcade games, retro gaming enthusiasts, and casual web gamers looking for quick entertainment sessions.",
            "pe_3": "High engagement time per session (>10 minutes average), positive user feedback, social sharing metrics, return visitor rate above 40%, and completion rate for games above 60%.",
            "pe_4": "Instant access to 10 classic games in one place, no downloads or installs required, works on any device, high scores saved locally, completely free with no ads or microtransactions.",

            # Phase 2: Technical Constraints
            "tc_1": "HTML5, CSS3, vanilla JavaScript ES6+ - no frameworks to keep it lightweight and fast loading",
            "tc_2": "Responsive design for mobile and desktop, 60 FPS gameplay for smooth animations, load time under 2 seconds, works offline with service workers for cached gameplay",
            "tc_3": "Client-side only application, no server load, designed to handle thousands of concurrent users through static hosting CDN, localStorage limited to 5-10MB per user for high scores",
            "tc_4": "No compliance requirements - fully client-side with no user data collection or transmission",

            # Phase 3: Integration Points
            "ip_1": "None - fully self-contained client-side application with no external API dependencies",
            "ip_2": "Browser localStorage API for persisting high scores and game state locally on the user's device",
            "ip_3": "No authentication required - all games playable immediately without login or user accounts",
            "ip_4": "No existing codebase - building from scratch as a standalone project",

            # Phase 4: Scope Definition
            "sd_1": "10 fully functional games (Snake, Pong, Breakout, Tetris, Space Invaders, Memory Match, 2048, Tic Tac Toe, Simon Says, Flappy Bird), main landing page with game selection grid, high score persistence per game, responsive UI for mobile and desktop, sound effects toggle, pause/resume functionality",
            "sd_2": "1-2 weeks for MVP with all 10 games fully functional and polished, another week for bug fixes and optimization",
            "sd_3": "Solo developer with AI assistance (Geoffrey autonomous agent), no budget constraints for static hosting (free tier)",
            "sd_4": "Core gameplay first, then polish and effects, mobile responsiveness throughout, high score persistence as essential feature",

            # Phase 5: Refinement & Validation
            "rv_1": "Yes, everything looks correct. Ready to proceed with architecture generation."
        }

        # Step 3: Submit all interview answers
        client.log("\n" + "=" * 80)
        client.log("STEP 3: Submit Interview Answers")
        client.log("=" * 80)

        question_count = 0
        max_questions = 25  # Safety limit

        while question_count < max_questions:
            question_count += 1

            # Parse current question from result
            if result and "result" in result:
                content = result["result"].get("content", [])
                if content:
                    text = content[0].get("text", "")
                    client.log(f"\n**Question {question_count}:**")
                    client.log(text[:300] + "..." if len(text) > 300 else text)

                    # Check if interview is complete
                    if "Interview Complete" in text or "interview ended" in text.lower():
                        client.log("\n✅ **Interview Complete!**")
                        break

                    # Extract question ID from text (format: "ID: pe_1")
                    question_id = None
                    for line in text.split('\n'):
                        if line.startswith("ID: "):
                            question_id = line.replace("ID: ", "").strip()
                            break

                    if question_id:
                        # Get answer for this question
                        answer = interview_answers.get(question_id, f"Comprehensive answer for {question_id}")

                        client.log(f"\n**Submitting answer for:** {question_id}")
                        client.log(f"**Answer:** {answer[:100]}...")

                        result = client.call_tool("submit_interview_answer", {
                            "projectPath": project_path,
                            "questionId": question_id,
                            "answer": answer
                        })
                    else:
                        client.log("\n⚠️ No question ID found in response")
                        break
                else:
                    break
            else:
                break

        # Step 4: Generate Architecture
        client.log("\n" + "=" * 80)
        client.log("STEP 4: Generate System Architecture")
        client.log("=" * 80)

        result = client.call_tool("generate_design", {
            "projectPath": project_path,
            "model": "glm-4.7"
        })

        if result and "result" in result:
            content = result["result"].get("content", [])
            if content:
                client.log(f"\n{content[0].get('text', 'Architecture generated')}")

        # Step 5: Create Development Plan
        client.log("\n" + "=" * 80)
        client.log("STEP 5: Create Development Plan")
        client.log("=" * 80)

        result = client.call_tool("create_devplan", {
            "projectPath": project_path,
            "model": "glm-4.7"
        })

        if result and "result" in result:
            content = result["result"].get("content", [])
            if content:
                client.log(f"\n{content[0].get('text', 'DevPlan created')}")

        # Step 6: Get phase list
        client.log("\n" + "=" * 80)
        client.log("STEP 6: List Development Phases")
        client.log("=" * 80)

        result = client.call_tool("list_phases", {
            "projectPath": project_path
        })

        # Step 7: Execute all phases
        client.log("\n" + "=" * 80)
        client.log("STEP 7: Execute Development Phases")
        client.log("=" * 80)

        # Execute phases 0 through 7
        for phase_num in range(8):
            phase_id = f"phase-{phase_num}"

            client.log(f"\n### Executing {phase_id}")
            client.log("-" * 60)

            result = client.call_tool("execute_phase", {
                "projectPath": project_path,
                "phaseId": phase_id,
                "model": "glm-4.7",
                "stopAfterPhase": True
            })

            if result and "result" in result:
                content = result["result"].get("content", [])
                if content:
                    client.log(f"\n{content[0].get('text', 'Phase executed')}")

                # Create checkpoint after each phase
                client.call_tool("create_checkpoint", {
                    "projectPath": project_path,
                    "name": f"after-{phase_id}"
                })

        # Step 8: Get final status
        client.log("\n" + "=" * 80)
        client.log("STEP 8: Final Project Status")
        client.log("=" * 80)

        result = client.call_tool("get_status", {
            "projectPath": project_path
        })

        if result and "result" in result:
            content = result["result"].get("content", [])
            if content:
                client.log(f"\n{content[0].get('text', 'Status retrieved')}")

        # Step 9: Get statistics
        client.log("\n" + "=" * 80)
        client.log("STEP 9: Token Usage & Cost Statistics")
        client.log("=" * 80)

        result = client.call_tool("get_stats", {
            "projectPath": project_path
        })

        if result and "result" in result:
            content = result["result"].get("content", [])
            if content:
                client.log(f"\n{content[0].get('text', 'Stats retrieved')}")

        client.log("\n" + "=" * 80)
        client.log("🎉 WORKFLOW COMPLETE!")
        client.log("=" * 80)
        client.log("\n✅ 10-Game Arcade Website built using ONLY MCP calls!")
        client.log(f"✅ Check {project_path} for the complete project")
        client.log(f"✅ Full log saved to MCP_COMPLETE_LOG.md")

    finally:
        client.close()

if __name__ == "__main__":
    main()
