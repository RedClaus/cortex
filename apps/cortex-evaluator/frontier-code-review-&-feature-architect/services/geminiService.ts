
import { GoogleGenAI, Type, GenerateContentParameters } from "@google/genai";
import { CodeFile, ReviewInput, AnalysisResult, SystemDocumentation } from "../types";

const SYSTEM_INSTRUCTION = `You are a Tier-1 Frontier System Architect. 
Your primary function is to evaluate external research (white papers, PDFs, or GitHub snippets) and determine its exact integration value for a private local codebase.

You are provided with:
1. System Documentation: An AI-generated overview of the target project's architecture.
2. Codebase Context: A structured manifest of the project files.

You must:
1. Use the System Documentation to understand high-level intent before deep-diving into code.
2. Produce a 'Change Request' (CR) that is strictly actionable. It should use the local project's existing naming conventions and design patterns.
3. The CR should be formatted for an AI Developer to execute immediately.

Output MUST be valid JSON according to the schema provided.`;

export async function analyzeWithGemini(
  codebase: CodeFile[],
  input: ReviewInput,
  systemDoc: SystemDocumentation | null
): Promise<AnalysisResult> {
  const ai = new GoogleGenAI({ apiKey: process.env.API_KEY });

  const codebaseManifest = codebase
    .map(f => `PATH: ${f.path}\nLANGUAGE: ${f.type}\nCONTENT:\n${f.content.substring(0, 10000)}`)
    .join('\n---\n');
  
  const textPrompt = `
    SYSTEM DOCUMENTATION:
    ${JSON.stringify(systemDoc, null, 2)}

    LOCAL CODEBASE CONTEXT:
    ${codebaseManifest}

    FRONTIER INPUT TO EVALUATE:
    NAME: ${input.name}
    TYPE: ${input.type}
    ${input.type !== 'pdf' ? `CONTENT:\n${input.content}` : 'The user has provided a PDF file (attached). Analyze its content against the codebase.'}

    OBJECTIVE:
    Analyze synergy between the Frontier Input and the existing System. 
    Generate a detailed Change Request (CR) in Markdown.
    
    CR MUST INCLUDE:
    - Summary of Changes
    - Affected Modules
    - Step-by-step Implementation Plan
    - Code Snippets for new or modified functions
  `;

  const parts: any[] = [{ text: textPrompt }];

  if (input.type === 'pdf' && input.fileData) {
    parts.push({
      inlineData: {
        mimeType: input.fileData.mimeType,
        data: input.fileData.data
      }
    });
  }

  try {
    const response = await ai.models.generateContent({
      model: "gemini-3-pro-preview",
      contents: { parts },
      config: {
        systemInstruction: SYSTEM_INSTRUCTION,
        responseMimeType: "application/json",
        responseSchema: {
          type: Type.OBJECT,
          properties: {
            valueScore: { type: Type.NUMBER },
            executiveSummary: { type: Type.STRING },
            technicalFeasibility: { type: Type.STRING },
            gapAnalysis: { type: Type.STRING },
            suggestedCR: { type: Type.STRING }
          },
          required: ["valueScore", "executiveSummary", "technicalFeasibility", "gapAnalysis", "suggestedCR"]
        }
      }
    });

    const text = response.text;
    if (!text) throw new Error("No response from architecture engine.");
    return JSON.parse(text) as AnalysisResult;
  } catch (error) {
    console.error("Architectural Synthesis Error:", error);
    throw error;
  }
}
