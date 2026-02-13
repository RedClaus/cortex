"""
Multi-Provider AI Router with Lane-Based Routing and Circuit Breaker

Routes requests to appropriate AI providers based on intent and constraints.
Implements fallback chains with circuit breaker protection.
"""
import asyncio
import logging
from abc import abstractmethod
from dataclasses import dataclass
from enum import Enum
from typing import Protocol, runtime_checkable, Optional

import anthropic
import google.generativeai as genai
import openai

from .circuit_breaker import CircuitBreaker, CircuitBreakerConfig
from .provider_configs import load_provider_configs

logger = logging.getLogger(__name__)


class RoutingLane(Enum):
    """Routing lanes for different request types."""
    FAST = "fast"      # Fast, cheap providers for simple tasks
    SMART = "smart"    # Smart, capable providers for complex tasks


@dataclass
class RoutingDecision:
    """Result of routing decision."""
    provider: "AIProvider"
    lane: RoutingLane
    reason: str


@runtime_checkable
class AIProvider(Protocol):
    """Abstract protocol for AI providers."""

    name: str
    lane: RoutingLane
    circuit_breaker: CircuitBreaker

    @abstractmethod
    async def analyze_code(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str] = None
    ) -> dict:
        """Analyze codebase and return evaluation results.

        Args:
            codebase: Dictionary of file paths to file contents
            input_data: User input and metadata
            system_doc: Optional system documentation

        Returns:
            Dictionary with analysis results (must include 'success' and 'result' keys)
        """
        ...

    @abstractmethod
    async def brainstorm(
        self,
        topic: str,
        constraints: Optional[list[str]] = None
    ) -> list[dict]:
        """Generate brainstorming ideas on a topic.

        Args:
            topic: Topic to brainstorm about
            constraints: Optional list of constraints to consider

        Returns:
            List of idea dictionaries with 'title' and 'description' keys
        """
        ...

    @abstractmethod
    async def expand_idea(
        self,
        idea: str,
        context: Optional[str] = None
    ) -> dict:
        """Expand on a specific idea with detailed analysis.

        Args:
            idea: The idea to expand on
            context: Optional context for the expansion

        Returns:
            Dictionary with 'title', 'description', 'considerations', and 'nextSteps' keys
        """
        ...

    @abstractmethod
    async def connect_ideas(
        self,
        idea_a: str,
        idea_b: str,
        relationship: str = "related"
    ) -> dict:
        """Analyze connection between two ideas.

        Args:
            idea_a: First idea
            idea_b: Second idea
            relationship: Type of relationship to analyze

        Returns:
            Dictionary with 'synergy', 'conflicts', 'complementary_aspects',
            'integration_approach', 'isValid', and 'bridgeConcept' keys
        """
        ...


class GeminiProvider:
    """Google Gemini provider implementation."""

    def __init__(self, config: dict[str, Optional[str]]):
        self.name = "gemini"
        self.lane = RoutingLane.FAST
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "gemini-1.5-pro")

        if self.api_key:
            genai.configure(api_key=self.api_key)
        self.client = genai.GenerativeModel(self.model)

        cb_config = CircuitBreakerConfig(failure_threshold=5, success_threshold=2)
        self.circuit_breaker = CircuitBreaker(self.name, cb_config)

    async def analyze_code(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str] = None
    ) -> dict:
        """Analyze codebase using Gemini."""

        async def _call():
            code_context = "\n\n".join(
                f"File: {path}\n{content[:2000]}"
                for path, content in list(codebase.items())[:5]
            )

            prompt = f"""Analyze this codebase:

{code_context}

User input: {input_data.get('query', '')}
"""

            response = self.client.generate_content(prompt)
            return {
                "success": True,
                "result": response.text,
                "provider": self.name,
                "model": self.model
            }

        return await self.circuit_breaker.call(_call)

    async def brainstorm(
        self,
        topic: str,
        constraints: Optional[list[str]] = None
    ) -> list[dict]:
        """Brainstorm using Gemini."""

        async def _call():
            constraint_text = "\n".join(f"- {c}" for c in (constraints or []))
            prompt = f"""Brainstorm ideas for: {topic}

Constraints:
{constraint_text or "None"}

Generate 5 ideas, each with a title and 2-3 sentence description.
"""

            response = self.client.generate_content(prompt)
            ideas = []

            for i, line in enumerate(response.text.split("\n")):
                if line.strip():
                    parts = line.split(":", 1)
                    title = parts[0].strip()
                    description = parts[1].strip() if len(parts) > 1 else ""
                    ideas.append({"title": title, "description": description})
                    if len(ideas) >= 5:
                        break

            return ideas

        return await self.circuit_breaker.call(_call)

    async def expand_idea(
        self,
        idea: str,
        context: Optional[str] = None
    ) -> dict:
        """Expand on an idea using Gemini."""

        async def _call():
            prompt = f"""Expand on this idea with detailed analysis:

Idea: {idea}
{f"Context: {context}" if context else ""}

Provide a structured response with:
1. A clear title summarizing the expanded idea
2. A detailed description (2-3 paragraphs) exploring the idea
3. 3-5 key considerations or potential challenges
4. 3-5 concrete next steps for implementation

Format your response as:
TITLE: [expanded title]
DESCRIPTION: [detailed description]
CONSIDERATIONS:
- [consideration 1]
- [consideration 2]
- [consideration 3]
NEXT STEPS:
- [step 1]
- [step 2]
- [step 3]
"""

            response = self.client.generate_content(prompt)
            text = response.text

            # Parse the response
            result = {
                "title": idea,
                "description": "",
                "considerations": [],
                "nextSteps": []
            }

            current_section = None
            for line in text.split("\n"):
                line = line.strip()
                if line.startswith("TITLE:"):
                    result["title"] = line[6:].strip()
                elif line.startswith("DESCRIPTION:"):
                    result["description"] = line[12:].strip()
                    current_section = "description"
                elif line.startswith("CONSIDERATIONS:"):
                    current_section = "considerations"
                elif line.startswith("NEXT STEPS:"):
                    current_section = "nextSteps"
                elif line.startswith("- ") and current_section in ["considerations", "nextSteps"]:
                    result[current_section].append(line[2:].strip())
                elif current_section == "description" and line:
                    result["description"] += " " + line

            return result

        return await self.circuit_breaker.call(_call)

    async def connect_ideas(
        self,
        idea_a: str,
        idea_b: str,
        relationship: str = "related"
    ) -> dict:
        """Analyze connection between ideas using Gemini."""

        async def _call():
            prompt = f"""Analyze the connection between these two ideas:

IDEA A: {idea_a}
IDEA B: {idea_b}
RELATIONSHIP TYPE: {relationship}

Provide a structured analysis:
1. SYNERGY: How well do these ideas work together? (1-2 sentences)
2. CONFLICTS: Any potential conflicts or contradictions? (1-2 sentences, or "None")
3. COMPLEMENTARY ASPECTS: List 2-3 ways they complement each other
4. INTEGRATION APPROACH: How to best combine or connect them (1-2 sentences)
5. IS VALID: Is this a logical/useful connection? (true/false)
6. BRIDGE CONCEPT: A single concept that bridges both ideas (1 sentence)

Format:
SYNERGY: [text]
CONFLICTS: [text]
COMPLEMENTARY:
- [aspect 1]
- [aspect 2]
INTEGRATION: [text]
VALID: [true/false]
BRIDGE: [bridging concept]
"""

            response = self.client.generate_content(prompt)
            text = response.text

            result = {
                "synergy": "",
                "conflicts": "",
                "complementary_aspects": [],
                "integration_approach": "",
                "isValid": True,
                "bridgeConcept": None
            }

            for line in text.split("\n"):
                line = line.strip()
                if line.startswith("SYNERGY:"):
                    result["synergy"] = line[8:].strip()
                elif line.startswith("CONFLICTS:"):
                    result["conflicts"] = line[10:].strip()
                elif line.startswith("- "):
                    result["complementary_aspects"].append(line[2:].strip())
                elif line.startswith("INTEGRATION:"):
                    result["integration_approach"] = line[12:].strip()
                elif line.startswith("VALID:"):
                    result["isValid"] = line[6:].strip().lower() == "true"
                elif line.startswith("BRIDGE:"):
                    result["bridgeConcept"] = line[7:].strip()

            return result

        return await self.circuit_breaker.call(_call)


class ClaudeProvider:
    """Anthropic Claude provider implementation."""

    def __init__(self, config: dict[str, Optional[str]]):
        self.name = "claude"
        self.lane = RoutingLane.SMART
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "claude-3-opus-20240229")

        self.client = anthropic.AsyncAnthropic(api_key=self.api_key)

        cb_config = CircuitBreakerConfig(failure_threshold=3, success_threshold=2)
        self.circuit_breaker = CircuitBreaker(self.name, cb_config)

    async def analyze_code(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str] = None
    ) -> dict:
        """Analyze codebase using Claude."""

        async def _call():
            code_context = "\n\n".join(
                f"File: {path}\n{content[:3000]}"
                for path, content in list(codebase.items())[:10]
            )

            system_msg = system_doc or "You are an expert code analyst."
            user_msg = f"""Analyze this codebase:

{code_context}

User input: {input_data.get('query', '')}

Provide comprehensive analysis including strengths, weaknesses, and recommendations.
"""

            response = await self.client.messages.create(
                model=self.model,
                max_tokens=4096,
                system=system_msg,
                messages=[{"role": "user", "content": user_msg}]
            )

            return {
                "success": True,
                "result": response.content[0].text,
                "provider": self.name,
                "model": self.model
            }

        return await self.circuit_breaker.call(_call)

    async def brainstorm(
        self,
        topic: str,
        constraints: Optional[list[str]] = None
    ) -> list[dict]:
        """Brainstorm using Claude."""

        async def _call():
            constraint_text = "\n".join(f"- {c}" for c in (constraints or []))
            prompt = f"""Brainstorm innovative ideas for: {topic}

Constraints:
{constraint_text or "None"}

Provide 5 distinct ideas. Format as:
1. Idea Title
   [2-3 sentence description]

2. Idea Title
   [2-3 sentence description]
"""

            response = await self.client.messages.create(
                model=self.model,
                max_tokens=2048,
                messages=[{"role": "user", "content": prompt}]
            )

            ideas = []
            text = response.content[0].text

            for match in text.split("\n\n"):
                if match.strip():
                    lines = match.split("\n")
                    if len(lines) >= 2:
                        title = lines[0].strip().lstrip("0123456789.- ")
                        description = "\n".join(lines[1:]).strip()
                        ideas.append({"title": title, "description": description})

            return ideas[:5]

        return await self.circuit_breaker.call(_call)

    async def expand_idea(
        self,
        idea: str,
        context: Optional[str] = None
    ) -> dict:
        """Expand on an idea using Claude."""

        async def _call():
            prompt = f"""Expand on this idea with detailed analysis:

Idea: {idea}
{f"Context: {context}" if context else ""}

Provide a structured response in JSON format with:
- title: A clear title summarizing the expanded idea
- description: A detailed description (2-3 paragraphs) exploring the idea
- considerations: An array of 3-5 key considerations or potential challenges
- nextSteps: An array of 3-5 concrete next steps for implementation

Return ONLY valid JSON, no other text."""

            response = await self.client.messages.create(
                model=self.model,
                max_tokens=2048,
                messages=[{"role": "user", "content": prompt}]
            )

            import json
            text = response.content[0].text

            try:
                # Try to parse as JSON
                return json.loads(text)
            except json.JSONDecodeError:
                # Fallback parsing
                return {
                    "title": idea,
                    "description": text,
                    "considerations": ["Further analysis recommended"],
                    "nextSteps": ["Review the detailed analysis above"]
                }

        return await self.circuit_breaker.call(_call)

    async def connect_ideas(
        self,
        idea_a: str,
        idea_b: str,
        relationship: str = "related"
    ) -> dict:
        """Analyze connection between ideas using Claude."""

        async def _call():
            prompt = f"""Analyze the connection between these two ideas and return JSON:

IDEA A: {idea_a}
IDEA B: {idea_b}
RELATIONSHIP TYPE: {relationship}

Return a JSON object with:
- synergy: How well do these ideas work together? (string)
- conflicts: Any potential conflicts? (string, or "None")
- complementary_aspects: Array of ways they complement each other
- integration_approach: How to best combine them (string)
- isValid: Is this a logical connection? (boolean)
- bridgeConcept: A concept that bridges both ideas (string)

Return ONLY valid JSON."""

            response = await self.client.messages.create(
                model=self.model,
                max_tokens=1024,
                messages=[{"role": "user", "content": prompt}]
            )

            import json
            try:
                return json.loads(response.content[0].text)
            except json.JSONDecodeError:
                return {
                    "synergy": "Connection analysis available",
                    "conflicts": "None identified",
                    "complementary_aspects": ["Potential synergy"],
                    "integration_approach": "Further analysis recommended",
                    "isValid": True,
                    "bridgeConcept": f"Link between {idea_a[:20]}... and {idea_b[:20]}..."
                }

        return await self.circuit_breaker.call(_call)


class OpenAIProvider:
    """OpenAI provider implementation."""

    def __init__(self, config: dict[str, Optional[str]]):
        self.name = "openai"
        self.lane = RoutingLane.FAST
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "gpt-4o")
        base_url = config.get("base_url")

        self.client = openai.AsyncOpenAI(api_key=self.api_key, base_url=base_url)

        cb_config = CircuitBreakerConfig(failure_threshold=5, success_threshold=2)
        self.circuit_breaker = CircuitBreaker(self.name, cb_config)

    async def analyze_code(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str] = None
    ) -> dict:
        """Analyze codebase using OpenAI."""

        async def _call():
            code_context = "\n\n".join(
                f"File: {path}\n{content[:2000]}"
                for path, content in list(codebase.items())[:5]
            )

            messages = [
                {"role": "system", "content": system_doc or "You are an expert code analyst."},
                {"role": "user", "content": f"Analyze this codebase:\n\n{code_context}\n\nUser input: {input_data.get('query', '')}"}
            ]

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=messages,
                max_tokens=2048
            )

            return {
                "success": True,
                "result": response.choices[0].message.content,
                "provider": self.name,
                "model": self.model
            }

        return await self.circuit_breaker.call(_call)

    async def brainstorm(
        self,
        topic: str,
        constraints: Optional[list[str]] = None
    ) -> list[dict]:
        """Brainstorm using OpenAI."""

        async def _call():
            constraint_text = "\n".join(f"- {c}" for c in (constraints or []))
            prompt = f"""Brainstorm ideas for: {topic}\n\nConstraints:\n{constraint_text or 'None'}\n\nGenerate 5 ideas as a JSON array with 'title' and 'description' fields."""

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                response_format={"type": "json_object"}
            )

            import json
            ideas = json.loads(response.choices[0].message.content)
            return ideas.get("ideas", ideas) if isinstance(ideas, dict) else ideas

        return await self.circuit_breaker.call(_call)

    async def expand_idea(
        self,
        idea: str,
        context: Optional[str] = None
    ) -> dict:
        """Expand on an idea using OpenAI."""

        async def _call():
            prompt = f"""Expand on this idea with detailed analysis:

Idea: {idea}
{f"Context: {context}" if context else ""}

Provide a structured response with:
- title: A clear title summarizing the expanded idea
- description: A detailed description (2-3 paragraphs) exploring the idea
- considerations: An array of 3-5 key considerations or potential challenges
- nextSteps: An array of 3-5 concrete next steps for implementation"""

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                response_format={"type": "json_object"},
                max_tokens=2048
            )

            import json
            return json.loads(response.choices[0].message.content)

        return await self.circuit_breaker.call(_call)

    async def connect_ideas(
        self,
        idea_a: str,
        idea_b: str,
        relationship: str = "related"
    ) -> dict:
        """Analyze connection between ideas using OpenAI."""

        async def _call():
            prompt = f"""Analyze the connection between these two ideas:

IDEA A: {idea_a}
IDEA B: {idea_b}
RELATIONSHIP TYPE: {relationship}

Return a JSON object with:
- synergy: How well do these ideas work together? (string)
- conflicts: Any potential conflicts? (string, or "None")
- complementary_aspects: Array of ways they complement each other
- integration_approach: How to best combine them (string)
- isValid: Is this a logical connection? (boolean)
- bridgeConcept: A concept that bridges both ideas (string)"""

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                response_format={"type": "json_object"},
                max_tokens=1024
            )

            import json
            return json.loads(response.choices[0].message.content)

        return await self.circuit_breaker.call(_call)


class OllamaProvider:
    """Ollama local provider implementation."""

    def __init__(self, config: dict[str, Optional[str]]):
        self.name = "ollama"
        self.lane = RoutingLane.FAST
        self.base_url = config.get("base_url", "http://localhost:11434")
        self.model = config.get("model", "llama3")

        self.client = openai.AsyncOpenAI(
            api_key="ollama",
            base_url=f"{self.base_url}/v1"
        )

        cb_config = CircuitBreakerConfig(failure_threshold=3, success_threshold=2)
        self.circuit_breaker = CircuitBreaker(self.name, cb_config)

    async def analyze_code(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str] = None
    ) -> dict:
        """Analyze codebase using Ollama."""

        async def _call():
            code_context = "\n\n".join(
                f"File: {path}\n{content[:1500]}"
                for path, content in list(codebase.items())[:3]
            )

            messages = [
                {"role": "system", "content": system_doc or "You are a code analyst."},
                {"role": "user", "content": f"Analyze:\n\n{code_context}\n\nInput: {input_data.get('query', '')}"}
            ]

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=messages,
                max_tokens=1024
            )

            return {
                "success": True,
                "result": response.choices[0].message.content,
                "provider": self.name,
                "model": self.model
            }

        return await self.circuit_breaker.call(_call)

    async def brainstorm(
        self,
        topic: str,
        constraints: Optional[list[str]] = None
    ) -> list[dict]:
        """Brainstorm using Ollama."""

        async def _call():
            constraint_text = "\n".join(f"- {c}" for c in (constraints or []))
            prompt = f"Brainstorm 5 ideas for: {topic}\n\nConstraints:\n{constraint_text or 'None'}"

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=512
            )

            ideas = []
            text = response.choices[0].message.content

            for line in text.split("\n"):
                if line.strip():
                    parts = line.split(":", 1)
                    ideas.append({
                        "title": parts[0].strip(),
                        "description": parts[1].strip() if len(parts) > 1 else ""
                    })
                    if len(ideas) >= 5:
                        break

            return ideas

        return await self.circuit_breaker.call(_call)

    async def expand_idea(
        self,
        idea: str,
        context: Optional[str] = None
    ) -> dict:
        """Expand on an idea using Ollama."""

        async def _call():
            prompt = f"""Expand on this idea with detailed analysis:

Idea: {idea}
{f"Context: {context}" if context else ""}

Provide:
1. TITLE: A clear title
2. DESCRIPTION: A detailed description (2-3 paragraphs)
3. CONSIDERATIONS: 3-5 key considerations (one per line, starting with -)
4. NEXT STEPS: 3-5 concrete next steps (one per line, starting with -)"""

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=1024
            )

            text = response.choices[0].message.content

            # Parse the response
            result = {
                "title": idea,
                "description": "",
                "considerations": [],
                "nextSteps": []
            }

            current_section = None
            for line in text.split("\n"):
                line = line.strip()
                if "TITLE:" in line.upper():
                    result["title"] = line.split(":", 1)[-1].strip()
                elif "DESCRIPTION:" in line.upper():
                    result["description"] = line.split(":", 1)[-1].strip()
                    current_section = "description"
                elif "CONSIDERATIONS:" in line.upper() or "CONSIDERATION:" in line.upper():
                    current_section = "considerations"
                elif "NEXT STEPS:" in line.upper() or "NEXT STEP:" in line.upper():
                    current_section = "nextSteps"
                elif line.startswith("- ") and current_section in ["considerations", "nextSteps"]:
                    result[current_section].append(line[2:].strip())
                elif current_section == "description" and line and not line.startswith(("1.", "2.", "3.", "4.")):
                    result["description"] += " " + line

            return result

        return await self.circuit_breaker.call(_call)

    async def connect_ideas(
        self,
        idea_a: str,
        idea_b: str,
        relationship: str = "related"
    ) -> dict:
        """Analyze connection between ideas using Ollama."""

        async def _call():
            prompt = f"""Analyze the connection between these two ideas:

IDEA A: {idea_a}
IDEA B: {idea_b}

Provide:
SYNERGY: How well do these ideas work together?
CONFLICTS: Any potential conflicts? (or "None")
COMPLEMENTARY: List ways they complement each other (starting with -)
INTEGRATION: How to best combine them
VALID: Is this logical? (true/false)
BRIDGE: A concept that bridges both ideas"""

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=512
            )

            text = response.choices[0].message.content
            result = {
                "synergy": "",
                "conflicts": "None",
                "complementary_aspects": [],
                "integration_approach": "",
                "isValid": True,
                "bridgeConcept": None
            }

            for line in text.split("\n"):
                line = line.strip()
                if "SYNERGY:" in line.upper():
                    result["synergy"] = line.split(":", 1)[-1].strip()
                elif "CONFLICTS:" in line.upper():
                    result["conflicts"] = line.split(":", 1)[-1].strip()
                elif line.startswith("- "):
                    result["complementary_aspects"].append(line[2:].strip())
                elif "INTEGRATION:" in line.upper():
                    result["integration_approach"] = line.split(":", 1)[-1].strip()
                elif "VALID:" in line.upper():
                    result["isValid"] = "true" in line.lower()
                elif "BRIDGE:" in line.upper():
                    result["bridgeConcept"] = line.split(":", 1)[-1].strip()

            return result

        return await self.circuit_breaker.call(_call)


class GroqProvider:
    """Groq provider implementation."""

    def __init__(self, config: dict[str, Optional[str]]):
        self.name = "groq"
        self.lane = RoutingLane.FAST
        self.api_key = config.get("api_key", "")
        self.model = config.get("model", "llama3-70b-8192")
        base_url = config.get("base_url", "https://api.groq.com/openai/v1")

        self.client = openai.AsyncOpenAI(api_key=self.api_key, base_url=base_url)

        cb_config = CircuitBreakerConfig(failure_threshold=5, success_threshold=2)
        self.circuit_breaker = CircuitBreaker(self.name, cb_config)

    async def analyze_code(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str] = None
    ) -> dict:
        """Analyze codebase using Groq."""

        async def _call():
            code_context = "\n\n".join(
                f"File: {path}\n{content[:2000]}"
                for path, content in list(codebase.items())[:5]
            )

            messages = [
                {"role": "system", "content": system_doc or "You are an expert code analyst."},
                {"role": "user", "content": f"Analyze:\n\n{code_context}\n\nInput: {input_data.get('query', '')}"}
            ]

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=messages,
                max_tokens=2048
            )

            return {
                "success": True,
                "result": response.choices[0].message.content,
                "provider": self.name,
                "model": self.model
            }

        return await self.circuit_breaker.call(_call)

    async def brainstorm(
        self,
        topic: str,
        constraints: Optional[list[str]] = None
    ) -> list[dict]:
        """Brainstorm using Groq."""

        async def _call():
            constraint_text = "\n".join(f"- {c}" for c in (constraints or []))
            prompt = f"Brainstorm 5 ideas for: {topic}\n\nConstraints:\n{constraint_text or 'None'}"

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=1024
            )

            ideas = []
            text = response.choices[0].message.content

            for line in text.split("\n"):
                if line.strip():
                    parts = line.split(":", 1)
                    ideas.append({
                        "title": parts[0].strip(),
                        "description": parts[1].strip() if len(parts) > 1 else ""
                    })
                    if len(ideas) >= 5:
                        break

            return ideas

        return await self.circuit_breaker.call(_call)

    async def expand_idea(
        self,
        idea: str,
        context: Optional[str] = None
    ) -> dict:
        """Expand on an idea using Groq."""

        async def _call():
            prompt = f"""Expand on this idea with detailed analysis:

Idea: {idea}
{f"Context: {context}" if context else ""}

Provide:
1. TITLE: A clear title
2. DESCRIPTION: A detailed description (2-3 paragraphs)
3. CONSIDERATIONS: 3-5 key considerations (one per line, starting with -)
4. NEXT STEPS: 3-5 concrete next steps (one per line, starting with -)"""

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=1024
            )

            text = response.choices[0].message.content

            # Parse the response
            result = {
                "title": idea,
                "description": "",
                "considerations": [],
                "nextSteps": []
            }

            current_section = None
            for line in text.split("\n"):
                line = line.strip()
                if "TITLE:" in line.upper():
                    result["title"] = line.split(":", 1)[-1].strip()
                elif "DESCRIPTION:" in line.upper():
                    result["description"] = line.split(":", 1)[-1].strip()
                    current_section = "description"
                elif "CONSIDERATIONS:" in line.upper() or "CONSIDERATION:" in line.upper():
                    current_section = "considerations"
                elif "NEXT STEPS:" in line.upper() or "NEXT STEP:" in line.upper():
                    current_section = "nextSteps"
                elif line.startswith("- ") and current_section in ["considerations", "nextSteps"]:
                    result[current_section].append(line[2:].strip())
                elif current_section == "description" and line and not line.startswith(("1.", "2.", "3.", "4.")):
                    result["description"] += " " + line

            return result

        return await self.circuit_breaker.call(_call)

    async def connect_ideas(
        self,
        idea_a: str,
        idea_b: str,
        relationship: str = "related"
    ) -> dict:
        """Analyze connection between two ideas using Groq."""

        async def _call():
            prompt = f"""Analyze the connection between these two ideas:

Idea A: {idea_a}
Idea B: {idea_b}
Relationship type: {relationship}

Provide analysis in this exact format:
SYNERGY: How these ideas complement each other
CONFLICTS: Potential tensions or contradictions
COMPLEMENTARY_ASPECTS: List 3-5 ways they complement (one per line, starting with -)
INTEGRATION_APPROACH: How to combine them effectively
IS_VALID: true or false - is this a logical connection?
BRIDGE_CONCEPT: A concept that links both ideas together"""

            response = await self.client.chat.completions.create(
                model=self.model,
                messages=[{"role": "user", "content": prompt}],
                max_tokens=1024
            )

            text = response.choices[0].message.content

            result = {
                "synergy": "",
                "conflicts": "",
                "complementary_aspects": [],
                "integration_approach": "",
                "isValid": True,
                "bridgeConcept": None
            }

            current_section = None
            for line in text.split("\n"):
                line = line.strip()
                upper_line = line.upper()
                if "SYNERGY:" in upper_line:
                    result["synergy"] = line.split(":", 1)[-1].strip()
                    current_section = "synergy"
                elif "CONFLICTS:" in upper_line or "CONFLICT:" in upper_line:
                    result["conflicts"] = line.split(":", 1)[-1].strip()
                    current_section = "conflicts"
                elif "COMPLEMENTARY_ASPECTS:" in upper_line or "COMPLEMENTARY ASPECTS:" in upper_line:
                    current_section = "complementary_aspects"
                elif "INTEGRATION_APPROACH:" in upper_line or "INTEGRATION APPROACH:" in upper_line:
                    result["integration_approach"] = line.split(":", 1)[-1].strip()
                    current_section = "integration"
                elif "IS_VALID:" in upper_line or "IS VALID:" in upper_line:
                    val = line.split(":", 1)[-1].strip().lower()
                    result["isValid"] = val in ["true", "yes", "1"]
                elif "BRIDGE_CONCEPT:" in upper_line or "BRIDGE CONCEPT:" in upper_line:
                    result["bridgeConcept"] = line.split(":", 1)[-1].strip()
                elif line.startswith("- ") and current_section == "complementary_aspects":
                    result["complementary_aspects"].append(line[2:].strip())

            return result

        return await self.circuit_breaker.call(_call)


class CortexRouter:
    """Multi-provider AI router with lane-based routing and fallback.

    Routes requests to FAST or SMART lane based on:
    1. Hard constraints (vision, context overflow)
    2. User intent (--strong, --local, --cheap)
    3. Default FAST lane
    """

    def __init__(self, config: Optional[dict] = None):
        """Initialize router with provider instances.

        Args:
            config: Optional configuration dict (uses env vars if None)
        """
        provider_configs = config or load_provider_configs()

        self.fast_lane: list[AIProvider] = [
            OllamaProvider(provider_configs["ollama"]),
            GroqProvider(provider_configs["groq"]),
            GeminiProvider(provider_configs["gemini"]),
        ]

        self.smart_lane: list[AIProvider] = [
            ClaudeProvider(provider_configs["claude"]),
            OpenAIProvider(provider_configs["openai"]),
        ]

        logger.info(
            f"CortexRouter initialized: FAST lane has {len(self.fast_lane)} providers, "
            f"SMART lane has {len(self.smart_lane)} providers"
        )

    def route_analysis(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str],
        user_intent: Optional[str]
    ) -> RoutingDecision:
        """Determine which provider to use for analysis.

        Args:
            codebase: Codebase files and contents
            input_data: User input and metadata
            system_doc: Optional system documentation
            user_intent: User intent flags (--strong, --local, --cheap)

        Returns:
            RoutingDecision with selected provider and lane
        """
        total_tokens = sum(len(content) for content in codebase.values())

        # Phase 1: Hard constraints
        if total_tokens > 100000:
            return RoutingDecision(
                provider=self.smart_lane[0],
                lane=RoutingLane.SMART,
                reason="Large context size requires SMART lane"
            )

        if input_data.get("has_vision"):
            return RoutingDecision(
                provider=self.smart_lane[0],
                lane=RoutingLane.SMART,
                reason="Vision capabilities required"
            )

        # Phase 2: User intent
        if user_intent == "--strong":
            return RoutingDecision(
                provider=self.smart_lane[0],
                lane=RoutingLane.SMART,
                reason="User requested strong analysis"
            )

        if user_intent == "--local":
            for provider in self.fast_lane:
                if provider.name == "ollama":
                    return RoutingDecision(
                        provider=provider,
                        lane=RoutingLane.FAST,
                        reason="User requested local provider"
                    )

        if user_intent == "--cheap":
            return RoutingDecision(
                provider=self.fast_lane[0],
                lane=RoutingLane.FAST,
                reason="User requested cheap provider"
            )

        # Phase 3: Default FAST lane
        return RoutingDecision(
            provider=self.fast_lane[0],
            lane=RoutingLane.FAST,
            reason="Default FAST lane"
        )

    async def analyze_with_fallback(
        self,
        codebase: dict[str, str],
        input_data: dict,
        system_doc: Optional[str],
        user_intent: Optional[str] = None
    ) -> dict:
        """Execute analysis with provider fallback chain.

        Args:
            codebase: Codebase files and contents
            input_data: User input and metadata
            system_doc: Optional system documentation
            user_intent: User intent flags

        Returns:
            Analysis result dict

        Raises:
            Exception: If all providers fail
        """
        decision = self.route_analysis(codebase, input_data, system_doc, user_intent)
        logger.info(f"Routing decision: {decision.reason} -> {decision.provider.name}")

        # Try primary lane
        for provider in (self.smart_lane if decision.lane == RoutingLane.SMART else self.fast_lane):
            if not self._validate_result(await provider.analyze_code(codebase, input_data, system_doc)):
                logger.warning(f"{provider.name} returned invalid result, skipping")
                continue

            return await provider.analyze_code(codebase, input_data, system_doc)

        # Fallback to opposite lane
        fallback_lane = self.fast_lane if decision.lane == RoutingLane.SMART else self.smart_lane
        for provider in fallback_lane:
            try:
                result = await provider.analyze_code(codebase, input_data, system_doc)
                if self._validate_result(result):
                    logger.info(f"Fallback to {provider.name} succeeded")
                    return result
            except Exception as e:
                logger.warning(f"Fallback to {provider.name} failed: {e}")
                continue

        raise Exception("All AI providers failed")

    def _validate_result(self, result: dict) -> bool:
        """Validate result has required fields.

        Args:
            result: Result dictionary to validate

        Returns:
            True if valid, False otherwise
        """
        required_fields = ["success", "result"]
        return all(field in result for field in required_fields) and result.get("success")

    def get_provider_status(self) -> dict[str, dict]:
        """Get status of all providers.

        Returns:
            Dict mapping provider names to status info
        """
        status = {}

        for provider in self.fast_lane + self.smart_lane:
            status[provider.name] = {
                "lane": provider.lane.value,
                "circuit_state": provider.circuit_breaker.state.value,
                "failure_rate": round(provider.circuit_breaker.failure_rate, 2)
            }

        return status
