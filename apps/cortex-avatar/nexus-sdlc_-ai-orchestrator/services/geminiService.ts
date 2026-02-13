
import { GoogleGenAI } from "@google/genai";
import { AICLI } from "../types";

const ai = new GoogleGenAI({ apiKey: process.env.API_KEY });

export async function getCLIResponse(cliType: AICLI, prompt: string, context: string = "") {
  const model = "gemini-3-pro-preview";
  
  const systemInstructions = `
    You are an advanced AI CLI interface named ${cliType}. 
    Your personality is concise, technical, and high-precision.
    Current Project Context: ${context}
    
    If the user gives you instructions, respond as if you are executing commands in a high-end development environment.
    Use ASCII art where appropriate for status updates.
    Be helpful in writing code, debugging, and managing files.
  `;

  try {
    const response = await ai.models.generateContent({
      model: model,
      contents: prompt,
      config: {
        systemInstruction: systemInstructions,
        temperature: 0.7,
      },
    });

    return response.text || "No response received from CLI.";
  } catch (error) {
    console.error("CLI Error:", error);
    return `[ERROR] Connection to ${cliType} failed. Check console for details.`;
  }
}

export async function runDiagnostics(code: string) {
  const model = "gemini-3-flash-preview";
  
  const prompt = `
    Perform a deep diagnostic scan of the following code.
    Identify security vulnerabilities, syntax errors, performance bottlenecks, and architectural issues.
    
    Return the response as a valid JSON object with the following structure:
    {
      "summary": "Short overview",
      "severity": "low|medium|high|critical",
      "findings": ["finding 1", "finding 2"],
      "suggestedFix": "Code block or detailed instructions for the CLI"
    }
  `;

  try {
    const response = await ai.models.generateContent({
      model: model,
      contents: [{ parts: [{ text: `${prompt}\n\nCODE:\n${code}` }] }],
      config: {
        responseMimeType: "application/json",
      },
    });

    return JSON.parse(response.text || "{}");
  } catch (error) {
    console.error("Diagnostic Error:", error);
    return {
      summary: "Error running diagnostics",
      severity: "critical",
      findings: ["Communication failure with diagnostic engine"],
      suggestedFix: "Retry scan manually."
    };
  }
}
