
import React, { useState, useCallback, useMemo, useEffect, useRef } from 'react';
import { AppState, Provider, CodeFile, ReviewInput, AnalysisResult, SystemDocumentation } from './types';
import { analyzeWithGemini } from './services/geminiService';
import { scanDirectory } from './services/contextStore';
import { fetchGitHubRepo } from './services/githubService';
import { generateSystemDocumentation } from './services/documentationService';
import { CodeIcon, FileIcon, RocketIcon, CheckIcon } from './components/Icons';

const PROVIDERS: { id: Provider; name: string; color: string }[] = [
  { id: 'gemini', name: 'Google Gemini', color: 'bg-blue-600' },
  { id: 'openai', name: 'OpenAI (Coming Soon)', color: 'bg-green-600' },
  { id: 'anthropic', name: 'Anthropic (Coming Soon)', color: 'bg-purple-600' },
  { id: 'groq', name: 'Groq (Coming Soon)', color: 'bg-orange-600' },
  { id: 'grok', name: 'xAI Grok (Coming Soon)', color: 'bg-gray-800' },
];

const App: React.FC = () => {
  const [isDarkMode, setIsDarkMode] = useState(true);
  const [state, setState] = useState<AppState>({
    codebase: [],
    inputs: [],
    selectedProvider: 'gemini',
    isAnalyzing: false,
    results: null,
    indexingStatus: {
      isIndexing: false,
      totalFiles: 0,
      processedFiles: 0,
      currentFile: ''
    },
    systemDocumentation: null
  });

  const [snippetText, setSnippetText] = useState('');
  const [githubUrl, setGithubUrl] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isGithubInputView, setIsGithubInputView] = useState(false);
  const [indexingPhase, setIndexingPhase] = useState<'scanning' | 'documenting' | 'vectorizing' | 'idle'>('idle');
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isDarkMode) {
      document.body.classList.add('dark');
      document.body.style.backgroundColor = '#000000';
    } else {
      document.body.classList.remove('dark');
      document.body.style.backgroundColor = '#f8fafc';
    }
  }, [isDarkMode]);

  const finalizeIndexing = async (files: CodeFile[]) => {
    setIndexingPhase('documenting');
    const systemDoc = await generateSystemDocumentation(files);
    setIndexingPhase('vectorizing');
    await new Promise(r => setTimeout(r, 1200));

    setState(prev => ({
      ...prev,
      codebase: files,
      systemDocumentation: systemDoc,
      indexingStatus: { ...prev.indexingStatus, isIndexing: false, totalFiles: files.length }
    }));
    setIndexingPhase('idle');
    setIsModalOpen(false);
    setIsGithubInputView(false);
  };

  const handleOpenFolder = async () => {
    try {
      // @ts-ignore
      const dirHandle = await window.showDirectoryPicker();
      setIndexingPhase('scanning');
      setState(prev => ({ 
        ...prev, 
        indexingStatus: { ...prev.indexingStatus, isIndexing: true, processedFiles: 0 } 
      }));
      const files = await scanDirectory(dirHandle, (current, count) => {
        setState(prev => ({
          ...prev,
          indexingStatus: { ...prev.indexingStatus, currentFile: current, processedFiles: count }
        }));
      });
      await finalizeIndexing(files);
    } catch (err: any) {
      if (err.name !== 'AbortError') setError("Failed to access folder.");
      setState(prev => ({ ...prev, indexingStatus: { ...prev.indexingStatus, isIndexing: false } }));
      setIndexingPhase('idle');
    }
  };

  const handleGitHubImport = async () => {
    if (!githubUrl.trim()) return;
    try {
      setIndexingPhase('scanning');
      setState(prev => ({ ...prev, indexingStatus: { ...prev.indexingStatus, isIndexing: true, processedFiles: 0 } }));
      const files = await fetchGitHubRepo(githubUrl, (current, count) => {
        setState(prev => ({
          ...prev,
          indexingStatus: { ...prev.indexingStatus, currentFile: current, processedFiles: count }
        }));
      });
      await finalizeIndexing(files);
      setGithubUrl('');
    } catch (err: any) {
      setError(err.message || "Failed to fetch GitHub repository.");
      setState(prev => ({ ...prev, indexingStatus: { ...prev.indexingStatus, isIndexing: false } }));
      setIndexingPhase('idle');
    }
  };

  const addSnippet = () => {
    if (!snippetText.trim()) return;
    setState(prev => ({
      ...prev,
      inputs: [...prev.inputs, { type: 'snippet', name: `Snippet ${prev.inputs.length + 1}`, content: snippetText }]
    }));
    setSnippetText('');
  };

  const handlePdfUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.type !== 'application/pdf') {
      setError("Only PDF files are supported.");
      return;
    }

    try {
      const base64 = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.readAsDataURL(file);
        reader.onload = () => resolve((reader.result as string).split(',')[1]);
        reader.onerror = error => reject(error);
      });

      setState(prev => ({
        ...prev,
        inputs: [...prev.inputs, { 
          type: 'pdf', 
          name: file.name, 
          content: `PDF Content: ${file.name}`,
          fileData: { data: base64, mimeType: file.type }
        }]
      }));
    } catch (err) {
      setError("Failed to read PDF.");
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const runAnalysis = async () => {
    if (state.codebase.length === 0) {
      setError("Please initialize project context first.");
      return;
    }
    if (state.inputs.length === 0) {
      setError("Please provide an evaluation input.");
      return;
    }
    setError(null);
    setState(prev => ({ ...prev, isAnalyzing: true, results: null }));
    try {
      const latestInput = state.inputs[state.inputs.length - 1];
      const result = await analyzeWithGemini(state.codebase, latestInput, state.systemDocumentation);
      setState(prev => ({ ...prev, results: result, isAnalyzing: false }));
    } catch (err: any) {
      setError(err.message || "An error occurred.");
      setState(prev => ({ ...prev, isAnalyzing: false }));
    }
  };

  const stats = useMemo(() => ({
    files: state.codebase.length,
    kb: Math.round(state.codebase.reduce((acc, f) => acc + f.content.length, 0) / 1024),
    langs: new Set(state.codebase.map(f => f.type)).size
  }), [state.codebase]);

  const closeModal = () => {
    if (!state.indexingStatus.isIndexing) {
      setIsModalOpen(false);
      setIsGithubInputView(false);
      setGithubUrl('');
    }
  };

  const themeClasses = {
    bg: isDarkMode ? 'bg-black' : 'bg-slate-50',
    card: isDarkMode ? 'bg-slate-900 border-slate-800' : 'bg-white border-slate-200 shadow-sm',
    header: 'bg-black border-slate-800', // Black navigation requested
    textPrimary: isDarkMode ? 'text-white' : 'text-slate-900',
    textSecondary: isDarkMode ? 'text-slate-400' : 'text-slate-500',
    input: isDarkMode ? 'bg-slate-950 border-slate-800 text-white' : 'bg-white border-slate-300 text-slate-900',
  };

  return (
    <div className={`min-h-screen ${themeClasses.bg} ${themeClasses.textPrimary} flex flex-col transition-colors duration-300`}>
      {/* Black Navigation Header */}
      <header className={`border-b ${themeClasses.header} sticky top-0 z-50`}>
        <div className="max-w-[1600px] mx-auto px-6 h-20 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 bg-blue-600 rounded-2xl flex items-center justify-center shadow-lg active:scale-95 transition-transform">
              <RocketIcon className="w-7 h-7 text-white" />
            </div>
            <div>
              <h1 className="text-xl font-black tracking-tight text-white">FRONTIER <span className="text-blue-500">ARCHITECT</span></h1>
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></div>
                <span className="text-[10px] uppercase font-bold tracking-tighter text-slate-400">System Ready • Quad-Color</span>
              </div>
            </div>
          </div>
          
          <div className="flex items-center gap-6">
            <div className="hidden xl:flex items-center gap-8 text-[10px] font-black uppercase tracking-widest border-x border-slate-800 px-8 py-2 text-slate-400">
               <div className="flex flex-col items-center"><span className="text-white text-sm">{stats.files}</span><span>Files</span></div>
               <div className="flex flex-col items-center"><span className="text-white text-sm">{stats.kb} KB</span><span>Context</span></div>
               <div className="flex flex-col items-center"><span className="text-white text-sm">{stats.langs}</span><span>Langs</span></div>
            </div>

            <button 
              onClick={() => setIsDarkMode(!isDarkMode)}
              className="p-2.5 rounded-xl border border-slate-800 bg-slate-900 text-yellow-500 hover:text-yellow-400 transition-all"
              title="Toggle Theme"
            >
              {isDarkMode ? (
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-5 h-5">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z" />
                </svg>
              ) : (
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-5 h-5">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.718 9.718 0 0118 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 003 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 009.002-5.998z" />
                </svg>
              )}
            </button>

            <div className="flex items-center gap-3 p-1.5 rounded-xl border border-slate-800 bg-slate-900">
              <span className="text-[10px] uppercase tracking-widest font-black text-slate-500 ml-3 mr-1">Provider</span>
              <select 
                className="bg-black text-white rounded-lg px-4 py-2 text-sm font-bold border border-slate-800 focus:ring-1 focus:ring-blue-500 outline-none cursor-pointer"
                value={state.selectedProvider}
                onChange={(e) => setState(prev => ({ ...prev, selectedProvider: e.target.value as Provider }))}
              >
                {PROVIDERS.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            </div>
          </div>
        </div>
      </header>

      <main className="flex-1 max-w-[1600px] mx-auto w-full p-6 grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* Left Column */}
        <div className="lg:col-span-4 space-y-6">
          <section className={`border rounded-[32px] p-6 relative overflow-hidden group ${themeClasses.card}`}>
            <div className="absolute top-0 right-0 p-8 opacity-5 group-hover:opacity-10 transition-opacity pointer-events-none text-blue-500"><CodeIcon className="w-32 h-32" /></div>
            <h2 className="text-xs font-black text-blue-500 uppercase tracking-widest mb-6 flex items-center gap-2">
              <CodeIcon className="w-4 h-4" />
              1. Context Engine
            </h2>
            <div className="space-y-4">
              <button 
                onClick={() => setIsModalOpen(true)}
                className={`w-full group/btn relative p-6 rounded-2xl flex flex-col items-center gap-3 transition-all border shadow-sm active:scale-[0.98] ${isDarkMode ? 'bg-slate-950 hover:bg-slate-800 border-slate-800' : 'bg-slate-50 hover:bg-slate-100 border-slate-200'}`}
              >
                <div className="w-12 h-12 bg-blue-600/10 rounded-xl flex items-center justify-center group-hover/btn:scale-110 transition-transform"><FileIcon className="w-6 h-6 text-blue-600" /></div>
                <div className="text-center"><span className="block text-sm font-bold">Initialize Project</span><span className="block text-[10px] text-blue-500 mt-1 uppercase tracking-wider font-bold">Files or GitHub</span></div>
              </button>
              {state.systemDocumentation && (
                <div className={`border p-4 rounded-xl ${isDarkMode ? 'bg-green-500/5 border-green-500/20' : 'bg-green-50 border-green-200'}`}>
                  <div className="flex items-center gap-2 mb-2">
                    <div className="w-2 h-2 rounded-full bg-green-500"></div>
                    <span className="text-[10px] font-black text-green-600 uppercase tracking-widest">Knowledge Base Ready</span>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {state.systemDocumentation.techStack.map((tech, i) => (
                      <span key={i} className={`text-[10px] px-2 py-1 rounded border font-black ${isDarkMode ? 'bg-black border-slate-800 text-slate-400' : 'bg-white border-slate-200 text-slate-600'}`}>{tech}</span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          </section>

          <section className={`border rounded-[32px] p-6 ${themeClasses.card}`}>
            <h2 className="text-xs font-black text-blue-500 uppercase tracking-widest mb-6 flex items-center gap-2"><FileIcon className="w-4 h-4" />2. Evaluate Input</h2>
            <div className="space-y-4">
              <div className="relative">
                <textarea 
                  placeholder="Paste research, papers, or snippets..."
                  className={`w-full rounded-2xl p-5 text-sm focus:ring-2 focus:ring-blue-500 outline-none h-40 resize-none transition-all placeholder:text-slate-500 font-medium ${themeClasses.input}`}
                  value={snippetText}
                  onChange={(e) => setSnippetText(e.target.value)}
                />
                <div className="absolute bottom-4 right-4 flex gap-2">
                  <input type="file" accept="application/pdf" className="hidden" ref={fileInputRef} onChange={handlePdfUpload} />
                  <button onClick={() => fileInputRef.current?.click()} className={`p-2.5 rounded-xl transition-all border shadow-sm ${isDarkMode ? 'bg-slate-800 hover:bg-slate-700 border-slate-700 text-slate-300' : 'bg-white hover:bg-slate-50 border-slate-300 text-slate-600'}`} title="Upload PDF"><FileIcon className="w-5 h-5" /></button>
                  <button onClick={addSnippet} disabled={!snippetText.trim()} className="px-5 py-2.5 bg-blue-600 hover:bg-blue-500 disabled:bg-slate-400 text-white rounded-xl text-xs font-black uppercase tracking-widest shadow-lg active:scale-95 transition-all">Queue Input</button>
                </div>
              </div>
              <div className="space-y-3">
                {state.inputs.map((input, i) => (
                  <div key={i} className={`flex items-center gap-4 p-3 rounded-2xl border group transition-colors ${isDarkMode ? 'bg-slate-950 border-slate-800 hover:border-blue-500/30' : 'bg-slate-50 border-slate-200 hover:border-blue-300'}`}>
                    <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${input.type === 'pdf' ? 'bg-yellow-500/10 text-yellow-500' : 'bg-blue-500/10 text-blue-500'}`}>
                      {input.type === 'pdf' ? <FileIcon className="w-4 h-4" /> : <CodeIcon className="w-4 h-4" />}
                    </div>
                    <div className="flex-1 min-w-0">
                      <span className="block text-xs font-bold truncate">{input.name}</span>
                      <span className={`block text-[10px] uppercase font-black tracking-tighter ${themeClasses.textSecondary}`}>{input.type === 'pdf' ? 'Frontier Paper' : 'Code Snippet'}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </section>

          {/* Yellow Alert Area if missing context */}
          {state.codebase.length === 0 && !state.isAnalyzing && (
            <div className="p-4 bg-yellow-500/10 border border-yellow-500/20 text-yellow-600 text-xs font-bold rounded-xl animate-pulse">
              ⚠️ Yellow Alert: No system context detected. Please initialize your knowledge base.
            </div>
          )}

          <button onClick={runAnalysis} disabled={state.isAnalyzing || state.inputs.length === 0 || state.codebase.length === 0}
            className={`w-full py-5 rounded-3xl font-black text-sm uppercase tracking-widest flex items-center justify-center gap-3 transition-all ${
              state.isAnalyzing 
                ? 'bg-slate-400 text-white cursor-not-allowed' 
                : 'bg-blue-600 hover:bg-blue-500 text-white shadow-xl shadow-blue-500/20 active:scale-95 border border-blue-400/20'
            }`}
          >
            {state.isAnalyzing ? <div className="flex items-center gap-3"><div className="w-5 h-5 border-2 border-white/20 border-t-white rounded-full animate-spin"></div>Processing Architecture...</div> : <><RocketIcon className="w-5 h-5" />Synthesize Blueprint</>}
          </button>
          
          {/* Red Error Area */}
          {error && (
            <div className="p-4 bg-red-500/10 border border-red-500/20 text-red-600 dark:text-red-400 text-xs font-bold rounded-xl animate-in fade-in">
              ❌ Critical Error: {error}
            </div>
          )}
        </div>

        {/* Right Column */}
        <div className="lg:col-span-8 space-y-8">
          {state.systemDocumentation && !state.results && !state.isAnalyzing && (
            <div className={`border rounded-[40px] p-10 animate-in fade-in slide-in-from-top-4 duration-500 ${themeClasses.card}`}>
              <div className="flex items-center gap-3 mb-6">
                <div className="w-2 h-8 bg-blue-600 rounded-full"></div>
                <h2 className="text-2xl font-black uppercase tracking-tight">System Knowledge Map</h2>
              </div>
              <div className="space-y-8">
                <div>
                  <h3 className="text-[10px] font-black text-blue-600 uppercase tracking-widest mb-2">Semantic Purpose</h3>
                  <p className={`leading-relaxed font-medium ${isDarkMode ? 'text-slate-300' : 'text-slate-700'}`}>{state.systemDocumentation.overview}</p>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                  <div>
                    <h3 className="text-[10px] font-black text-blue-600 uppercase tracking-widest mb-2">Architectural Layering</h3>
                    <p className={`text-sm leading-relaxed ${themeClasses.textSecondary}`}>{state.systemDocumentation.architecture}</p>
                  </div>
                  <div className={`p-6 rounded-3xl border ${isDarkMode ? 'bg-black border-slate-800' : 'bg-slate-50 border-slate-200'}`}>
                    <h3 className="text-[10px] font-black text-blue-600 uppercase tracking-widest mb-4">Module Inventory</h3>
                    <ul className="space-y-3">
                      {state.systemDocumentation.keyModules.slice(0, 5).map((m, i) => (
                        <li key={i} className="text-xs flex items-start gap-3">
                          <span className="text-green-500 mt-1">✔</span>
                          <div>
                            <span className="font-black block mb-0.5">{m.name}</span>
                            <p className={`text-[10px] leading-relaxed ${themeClasses.textSecondary}`}>{m.responsibility}</p>
                          </div>
                        </li>
                      ))}
                    </ul>
                  </div>
                </div>
              </div>
            </div>
          )}

          {!state.results && !state.isAnalyzing && !state.systemDocumentation && (
            <div className={`h-full min-h-[600px] flex flex-col items-center justify-center text-center p-12 border-2 border-dashed rounded-[40px] group transition-colors ${isDarkMode ? 'bg-slate-900/10 border-slate-800 hover:border-blue-500/20' : 'bg-white border-slate-200 hover:border-blue-300'}`}>
              <div className="relative mb-8">
                <div className="absolute inset-0 bg-blue-600/10 blur-3xl rounded-full group-hover:bg-blue-600/20 transition-colors"></div>
                <div className={`relative w-24 h-24 rounded-[32px] flex items-center justify-center border transition-colors ${isDarkMode ? 'bg-slate-900 border-slate-800 group-hover:border-blue-600/50' : 'bg-slate-50 border-slate-300 group-hover:border-blue-500'}`}>
                  <CodeIcon className={`w-10 h-10 transition-colors ${isDarkMode ? 'text-slate-700 group-hover:text-blue-500' : 'text-slate-300 group-hover:text-blue-600'}`} />
                </div>
              </div>
              <h2 className="text-3xl font-black mb-4">Frontier Architect</h2>
              <p className={`max-w-sm text-sm leading-relaxed font-medium ${themeClasses.textSecondary}`}>Map your local environment to begin automated architectural evaluation of frontier research.</p>
            </div>
          )}

          {state.isAnalyzing && (
            <div className={`h-full min-h-[600px] flex flex-col items-center justify-center text-center space-y-8 rounded-[40px] border transition-colors ${isDarkMode ? 'bg-black border-slate-900' : 'bg-white border-slate-100 shadow-inner'}`}>
               <div className="relative">
                  <div className="w-32 h-32 border-[6px] border-blue-600/10 border-t-blue-600 rounded-full animate-spin"></div>
                  <div className="absolute inset-0 flex items-center justify-center"><RocketIcon className="w-10 h-10 text-blue-600 animate-pulse" /></div>
               </div>
               <div className="space-y-3">
                 <h2 className="text-2xl font-black uppercase tracking-widest">Synthesizing Integration</h2>
                 <p className="text-blue-600 text-sm font-black uppercase tracking-widest italic animate-pulse">Running semantic cross-analysis...</p>
               </div>
            </div>
          )}

          {state.results && (
            <div className="space-y-8 animate-in fade-in slide-in-from-bottom-8 duration-700">
              <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <div className={`border p-8 rounded-[32px] flex flex-col items-center justify-center shadow-2xl relative overflow-hidden group ${themeClasses.card}`}>
                  <div className="absolute inset-0 bg-gradient-to-br from-yellow-500/10 to-transparent opacity-0 group-hover:opacity-100 transition-opacity"></div>
                  <span className="text-[10px] font-black text-yellow-600 uppercase tracking-[0.2em] mb-4">Strategic Value</span>
                  <div className="relative text-6xl font-black bg-gradient-to-b from-yellow-500 to-yellow-800 bg-clip-text text-transparent">{state.results.valueScore}</div>
                </div>
                <div className={`md:col-span-3 border p-8 rounded-[32px] shadow-2xl ${themeClasses.card}`}>
                   <h3 className="text-xs font-black text-blue-600 uppercase tracking-[0.2em] mb-4 flex items-center gap-2"><CheckIcon className="w-4 h-4 text-green-500" />Architectural Verdict</h3>
                   <p className={`text-base leading-relaxed font-medium italic ${isDarkMode ? 'text-slate-300' : 'text-slate-700'}`}>"{state.results.executiveSummary}"</p>
                </div>
              </div>
              
              <div className={`rounded-[40px] border overflow-hidden shadow-2xl ${isDarkMode ? 'bg-slate-950 border-slate-800' : 'bg-white border-slate-200'}`}>
                <div className={`px-8 py-6 border-b flex items-center justify-between ${isDarkMode ? 'bg-black border-slate-800' : 'bg-slate-50 border-slate-200'}`}>
                  <div className="flex items-center gap-4">
                    <div className="w-12 h-12 rounded-2xl bg-blue-600 flex items-center justify-center shadow-lg"><CodeIcon className="w-6 h-6 text-white" /></div>
                    <div>
                      <h3 className="font-black tracking-tight uppercase text-sm">Actionable CR Protocol</h3>
                      <span className={`text-[10px] font-bold uppercase tracking-widest ${themeClasses.textSecondary}`}>Implementation Blueprint</span>
                    </div>
                  </div>
                  <button onClick={() => { navigator.clipboard.writeText(state.results?.suggestedCR || ''); alert('Copied!'); }} className="bg-blue-600 hover:bg-blue-500 text-white px-6 py-3 rounded-2xl transition-all font-black text-xs uppercase tracking-widest shadow-md active:scale-95">Copy Blueprint</button>
                </div>
                <div className="p-8">
                  <div className={`border rounded-3xl p-8 overflow-x-auto shadow-inner ${isDarkMode ? 'bg-black border-slate-800' : 'bg-slate-50 border-slate-300'}`}>
                    <pre className={`text-xs font-mono whitespace-pre-wrap leading-relaxed selection:bg-blue-500/50 ${isDarkMode ? 'text-slate-300' : 'text-slate-800'}`}>{state.results.suggestedCR}</pre>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </main>

      {/* Modal - Context Configurator */}
      {isModalOpen && (
        <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 bg-black/90 backdrop-blur-md animate-in fade-in duration-300">
          <div className={`w-full max-w-2xl border rounded-[40px] shadow-2xl overflow-hidden relative ${isDarkMode ? 'bg-slate-900 border-slate-800' : 'bg-white border-slate-200'}`}>
            <button onClick={closeModal} className={`absolute top-8 right-8 transition-colors p-2 ${themeClasses.textSecondary} hover:text-red-500`}><span className="text-2xl">×</span></button>
            <div className="p-12">
              <div className="flex items-center gap-4 mb-8">
                <div className="w-14 h-14 bg-blue-600 rounded-2xl flex items-center justify-center"><RocketIcon className="w-8 h-8 text-white" /></div>
                <div><h2 className="text-2xl font-black uppercase">Context Architect</h2><p className={`text-sm font-medium italic ${themeClasses.textSecondary}`}>Establish semantic baseline</p></div>
              </div>
              {indexingPhase === 'idle' ? (
                <>
                  {!isGithubInputView ? (
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 animate-in fade-in slide-in-from-bottom-4 duration-300">
                      <button onClick={handleOpenFolder} className={`group p-8 rounded-3xl border transition-all text-left flex flex-col gap-4 ${isDarkMode ? 'bg-black hover:bg-blue-600 border-slate-800 hover:border-blue-400' : 'bg-slate-50 hover:bg-blue-500 border-slate-200 hover:border-blue-400'}`}>
                        <div className={`w-12 h-12 rounded-2xl flex items-center justify-center transition-colors ${isDarkMode ? 'bg-slate-900 group-hover:bg-white/10' : 'bg-white group-hover:bg-white/20'}`}><FileIcon className="w-6 h-6 text-blue-600 group-hover:text-white" /></div>
                        <div><h3 className={`font-black mb-1 group-hover:text-white`}>Local Scan</h3><p className={`text-xs italic group-hover:text-blue-100 ${themeClasses.textSecondary}`}>Direct access</p></div>
                      </button>
                      <button onClick={() => setIsGithubInputView(true)} className={`group p-8 rounded-3xl border transition-all text-left flex flex-col gap-4 ${isDarkMode ? 'bg-black hover:bg-blue-600 border-slate-800 hover:border-blue-400' : 'bg-slate-50 hover:bg-blue-500 border-slate-200 hover:border-blue-400'}`}>
                        <div className={`w-12 h-12 rounded-2xl flex items-center justify-center transition-colors ${isDarkMode ? 'bg-slate-900 group-hover:bg-white/10' : 'bg-white group-hover:bg-white/20'}`}><CodeIcon className="w-6 h-6 text-blue-600 group-hover:text-white" /></div>
                        <div><h3 className={`font-black mb-1 group-hover:text-white`}>GitHub API</h3><p className={`text-xs italic group-hover:text-blue-100 ${themeClasses.textSecondary}`}>Remote repo</p></div>
                      </button>
                    </div>
                  ) : (
                    <div className="space-y-6 animate-in fade-in slide-in-from-right-4 duration-300">
                      <div className="space-y-2"><label className="text-[10px] font-black text-slate-500 uppercase tracking-widest">Repository URL</label><input type="text" placeholder="https://github.com/owner/repo" className={`w-full rounded-2xl p-5 text-sm focus:ring-1 focus:ring-blue-500 outline-none transition-all placeholder:text-slate-600 font-bold ${themeClasses.input}`} value={githubUrl} onChange={(e) => setGithubUrl(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && handleGitHubImport()} /></div>
                      <div className="flex gap-4"><button onClick={() => setIsGithubInputView(false)} className={`flex-1 py-4 rounded-2xl text-xs font-black uppercase tracking-widest transition-all ${isDarkMode ? 'bg-slate-800 hover:bg-slate-700' : 'bg-slate-100 hover:bg-slate-200'}`}>Back</button><button onClick={handleGitHubImport} disabled={!githubUrl.trim()} className="flex-[2] py-4 bg-blue-600 hover:bg-blue-500 disabled:bg-slate-400 text-white rounded-2xl text-xs font-black uppercase tracking-widest transition-all shadow-lg">Start Mapping</button></div>
                    </div>
                  )}
                </>
              ) : (
                <div className="space-y-10 py-10 animate-in fade-in duration-500">
                  <div className="space-y-4">
                    <div className="flex items-center justify-between text-[10px] font-black uppercase tracking-widest">
                      <span className={indexingPhase === 'scanning' ? 'text-blue-500' : 'text-slate-500'}>{indexingPhase === 'scanning' && '• '} 1. Parsing</span>
                      <span className={indexingPhase === 'documenting' ? 'text-green-500' : 'text-slate-500'}>{indexingPhase === 'documenting' && '• '} 2. Semantic Analysis</span>
                      <span className={indexingPhase === 'vectorizing' ? 'text-yellow-500' : 'text-slate-500'}>{indexingPhase === 'vectorizing' && '• '} 3. Vectorization</span>
                    </div>
                    <div className={`w-full h-1.5 rounded-full overflow-hidden ${isDarkMode ? 'bg-black' : 'bg-slate-200'}`}><div className={`h-full transition-all duration-700 ease-out ${indexingPhase === 'scanning' ? 'bg-blue-600 w-[33%]' : indexingPhase === 'documenting' ? 'bg-green-500 w-[66%]' : 'bg-yellow-500 w-[100%]'}`}></div></div>
                  </div>
                  <div className="text-center space-y-2">
                    <div className="text-sm font-black uppercase tracking-tight">{indexingPhase === 'scanning' && `Parsing Structure... (${state.indexingStatus.processedFiles} files)`}{indexingPhase === 'documenting' && 'Generating System Documentation...'}{indexingPhase === 'vectorizing' && 'Committing to Frontier Vector Store...'}</div>
                    <div className="text-[10px] font-mono text-slate-500 truncate max-w-xs mx-auto italic font-medium">{state.indexingStatus.currentFile || 'Initializing pipeline...'}</div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default App;
