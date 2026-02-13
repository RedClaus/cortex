
import { GoogleGenAI, Type } from "@google/genai";
import { CodeFile, SystemDocumentation } from "../types";

export async function generateSystemDocumentation(codebase: CodeFile[]): Promise<SystemDocumentation> {
  const ai = new GoogleGenAI({ apiKey: process.env.API_KEY });
  
  // Create a condensed map of the project structure
  const projectMap = codebase.map(f => f.path).join('\n');
  const sampleContent = codebase
    .filter(f => !f.path.includes('test') && !f.path.includes('spec'))
    .slice(0, 15) // Take a representative sample of files
    .map(f => `FILE: ${f.path}\nCONTENT START:\n${f.content.substring(0, 1000)}`)
    .join('\n\n');

  const prompt = `
    Analyze the following project structure and sample file contents. 
    Provide a comprehensive architectural overview and semantic documentation.
    
    PROJECT STRUCTURE:
    ${projectMap}
    
    REPRESENTATIVE SAMPLES:
    ${sampleContent}
    
    Return the analysis in valid JSON format matching this schema:
    {
      "overview": "General purpose of the system",
      "architecture": "High-level design patterns and flow",
      "keyModules": [{"name": "module path", "responsibility": "what it does"}],
      "techStack": ["list of technologies identified"]
    }
  `;

  try {
    const response = await ai.models.generateContent({
      model: "gemini-3-flash-preview", // Use flash for fast indexing
      contents: prompt,
      config: {
        responseMimeType: "application/json",
        responseSchema: {
          type: Type.OBJECT,
          properties: {
            overview: { type: Type.STRING },
            architecture: { type: Type.STRING },
            keyModules: {
              type: Type.ARRAY,
              items: {
                type: Type.OBJECT,
                properties: {
                  name: { type: Type.STRING },
                  responsibility: { type: Type.STRING }
                },
                required: ["name", "responsibility"]
              }
            },
            techStack: {
              type: Type.ARRAY,
              items: { type: Type.STRING }
            }
          },
          required: ["overview", "architecture", "keyModules", "techStack"]
        }
      }
    });

    const text = response.text;
    if (!text) throw new Error("Documentation failed");
    return JSON.parse(text) as SystemDocumentation;
  } catch (error) {
    console.error("Documentation Generation Error:", error);
    // Return a fallback documentation object
    return {
      overview: "Generic code project",
      architecture: "Not determined",
      keyModules: [],
      techStack: []
    };
  }
}
