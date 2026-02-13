
import React, { useState, useEffect } from 'react';
import { 
  LayoutDashboard, 
  FlaskConical, 
  Code2, 
  Stethoscope, 
  Rocket, 
  Archive, 
  Brain, 
  Bell, 
  Settings,
  Menu,
  X,
  Plus,
  ArrowRight,
  ShieldCheck,
  Zap,
  Activity,
  Box,
  ChevronRight,
  ClipboardCheck,
  History,
  Save,
  Trash2
} from 'lucide-react';
import { SDLCStage, AICLI, BrainSession, Blueprint, BrainSnippet } from './types';
import { Terminal } from './components/Terminal';
import { brainService } from './services/brainService';
import { runDiagnostics } from './services/geminiService';

const App: React.FC = () => {
  const [currentStage, setCurrentStage] = useState<SDLCStage>(SDLCStage.WORKSHOP);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [brainPanelOpen, setBrainPanelOpen] = useState(false);
  const [session, setSession] = useState<BrainSession | null>(null);
  const [doctorCode, setDoctorCode] = useState('');
  const [isScanning, setIsScanning] = useState(false);
  const [diagReport, setDiagReport] = useState<any>(null);
  const [blueprints, setBlueprints] = useState<Blueprint[]>([
    { id: 'bp-1', title: 'NextGen Auth flow', description: 'OIDC integration with custom JWT validation.', tags: ['security', 'auth'] },
    { id: 'bp-2', title: 'A2A Protocol extension', description: 'Adding multi-factor session state syncing.', tags: ['network', 'brain'] }
  ]);
  const [deployStatus, setDeployStatus] = useState<'idle' | 'deploying' | 'live'>('idle');

  // New Snippet Form State
  const [isAddingSnippet, setIsAddingSnippet] = useState(false);
  const [newSnipTitle, setNewSnipTitle] = useState('');
  const [newSnipCode, setNewSnipCode] = useState('');
  const [newSnipLang, setNewSnipLang] = useState('typescript');

  useEffect(() => {
    const initBrain = async () => {
      const auth = await brainService.authenticate();
      if (auth.success) {
        const stats = await brainService.getSessionStats();
        setSession({
          sessionId: auth.sessionId,
          authStatus: 'authenticated',
          runbacks: [
            { id: 'rb-1', command: 'nexus init-sync --force', timestamp: Date.now() - 100000 },
            { id: 'rb-2', command: 'brain deploy --env=prod', timestamp: Date.now() - 50000 },
          ],
          codeSnippets: [
            { id: 'snip-1', title: 'A2A Auth Hook', language: 'typescript', code: 'const useAuth = () => brain.sync();' }
          ],
          stats
        });
      }
    };
    initBrain();
  }, []);

  const handleScan = async () => {
    if (!doctorCode.trim()) return;
    setIsScanning(true);
    const result = await runDiagnostics(doctorCode);
    setDiagReport(result);
    setIsScanning(false);
  };

  const handleDeploy = () => {
    setDeployStatus('deploying');
    setTimeout(() => setDeployStatus('live'), 3000);
  };

  const handleAddSnippet = async () => {
    if (!newSnipTitle || !newSnipCode || !session) return;

    const newSnippet: BrainSnippet = {
      id: `snip-${Date.now()}`,
      title: newSnipTitle,
      code: newSnipCode,
      language: newSnipLang
    };

    const updatedSnippets = [...session.codeSnippets, newSnippet];
    const updatedSession = { ...session, codeSnippets: updatedSnippets };
    
    setSession(updatedSession);
    await brainService.syncSession({ codeSnippets: updatedSnippets });

    // Reset form
    setNewSnipTitle('');
    setNewSnipCode('');
    setIsAddingSnippet(false);
  };

  const handleDeleteSnippet = async (id: string) => {
    if (!session) return;
    const updatedSnippets = session.codeSnippets.filter(s => s.id !== id);
    const updatedSession = { ...session, codeSnippets: updatedSnippets };
    setSession(updatedSession);
    await brainService.syncSession({ codeSnippets: updatedSnippets });
  };

  const navItems = [
    { stage: SDLCStage.WORKSHOP, icon: FlaskConical, label: 'Workshop', color: 'text-amber-400', path: 'workshop' },
    { stage: SDLCStage.DEVELOPMENT, icon: Code2, label: 'Development', color: 'text-emerald-400', path: 'dev' },
    { stage: SDLCStage.TEST, icon: Rocket, label: 'Test Suite', color: 'text-sky-400', path: 'test' },
    { stage: SDLCStage.PRODUCTION, icon: LayoutDashboard, label: 'Production', color: 'text-rose-400', path: 'production' },
    { stage: SDLCStage.ARCHIVE, icon: Archive, label: 'Archive', color: 'text-zinc-500', path: 'archive' },
    { stage: SDLCStage.DOCTOR, icon: Stethoscope, label: 'Doctor', color: 'text-red-400', path: 'doctor' },
  ];

  const renderContent = () => {
    switch (currentStage) {
      case SDLCStage.DEVELOPMENT:
        return (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 h-full animate-in fade-in duration-500 overflow-y-auto pr-2 pb-2">
            <Terminal type={AICLI.GEMINI} />
            <Terminal type={AICLI.CLAUDE} />
            <Terminal type={AICLI.OPENCODE} />
            <Terminal type={AICLI.DEEPSEEK} />
          </div>
        );

      case SDLCStage.WORKSHOP:
        return (
          <div className="flex flex-col gap-6 p-6 animate-in slide-in-from-bottom-4 duration-500">
            <div className="flex justify-between items-center">
              <div>
                <h2 className="text-3xl font-bold text-amber-100 flex items-center gap-3">
                  <FlaskConical className="text-amber-500" />
                  Idea Workshop
                </h2>
                <p className="text-zinc-500">Blueprint your project root components before initialization.</p>
              </div>
              <button 
                onClick={() => setBlueprints([...blueprints, { id: `bp-${Date.now()}`, title: 'New Concept', description: 'Pending description...', tags: ['new'] }])}
                className="flex items-center gap-2 bg-amber-600/20 text-amber-400 border border-amber-600/30 px-4 py-2 rounded-lg hover:bg-amber-600/30 transition-all"
              >
                <Plus size={18} /> New Blueprint
              </button>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {blueprints.map(bp => (
                <div key={bp.id} className="bg-zinc-900/40 border border-zinc-800 p-6 rounded-2xl hover:border-amber-500/50 hover:bg-zinc-900/60 transition-all cursor-pointer group relative overflow-hidden">
                  <div className="absolute top-0 right-0 w-24 h-24 bg-amber-500/5 blur-3xl rounded-full" />
                  <div className="flex justify-between items-start mb-4">
                    <div className="w-12 h-12 rounded-xl bg-amber-500/10 flex items-center justify-center text-amber-500 group-hover:scale-110 transition-transform">
                      <Box size={24} />
                    </div>
                    <div className="flex gap-1">
                      {bp.tags.map(tag => (
                        <span key={tag} className="text-[10px] uppercase font-bold tracking-widest bg-zinc-800 text-zinc-500 px-2 py-0.5 rounded-full">{tag}</span>
                      ))}
                    </div>
                  </div>
                  <h3 className="text-xl font-bold text-zinc-100 group-hover:text-amber-400 transition-colors">{bp.title}</h3>
                  <p className="text-sm text-zinc-500 mt-3 leading-relaxed">{bp.description}</p>
                  <div className="mt-6 flex items-center justify-between border-t border-zinc-800 pt-4">
                    <span className="text-xs text-zinc-600 font-mono italic">#BLUEPRINT-{bp.id}</span>
                    <button 
                      onClick={() => setCurrentStage(SDLCStage.DEVELOPMENT)}
                      className="text-amber-500 hover:text-amber-400 flex items-center gap-1 text-sm font-bold"
                    >
                      Initialize Dev <ArrowRight size={16} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        );

      case SDLCStage.TEST:
        return (
          <div className="flex flex-col h-full gap-6 p-6 animate-in zoom-in-95 duration-500">
            <div className="flex items-center justify-between border-b border-zinc-800 pb-6">
              <div>
                <h2 className="text-3xl font-bold text-sky-100 flex items-center gap-3">
                  <Rocket className="text-sky-500" /> Test Suite Console
                </h2>
                <p className="text-zinc-500">Running automated validations in the PROJECT_ROOT/test sandbox.</p>
              </div>
              <div className="flex gap-4">
                <div className="bg-zinc-900 border border-zinc-800 p-3 rounded-lg flex flex-col items-center min-w-[100px]">
                  <span className="text-[10px] text-zinc-600 font-bold uppercase">Pass Rate</span>
                  <span className="text-emerald-500 text-lg font-bold">98.2%</span>
                </div>
                <div className="bg-zinc-900 border border-zinc-800 p-3 rounded-lg flex flex-col items-center min-w-[100px]">
                  <span className="text-[10px] text-zinc-600 font-bold uppercase">Coverage</span>
                  <span className="text-sky-500 text-lg font-bold">84%</span>
                </div>
              </div>
            </div>
            
            <div className="grid grid-cols-1 lg:grid-cols-4 gap-6 flex-1">
              <div className="lg:col-span-3 bg-black border border-zinc-800 rounded-2xl overflow-hidden flex flex-col shadow-inner">
                <div className="bg-zinc-900/50 p-4 border-b border-zinc-800 flex items-center gap-3">
                  <Activity size={18} className="text-sky-500" />
                  <span className="text-xs font-bold font-mono text-zinc-400">UNIT_TEST_RUNNER v4.1 (Nexus Native)</span>
                </div>
                <div className="flex-1 p-4 font-mono text-sm overflow-y-auto space-y-2 bg-[#050506]">
                  <p className="text-zinc-500">[08:45:01] Initializing Test Suite...</p>
                  <p className="text-emerald-500">✓ Auth Integration (24ms)</p>
                  <p className="text-emerald-500">✓ Brain A2A Handshake (112ms)</p>
                  <p className="text-emerald-500">✓ Project Root File Permissions (1ms)</p>
                  <p className="text-red-400">✗ Legacy API Deprecation Warning (Found 3 references)</p>
                  <p className="text-emerald-500">✓ Diagnostic Scanner Core (54ms)</p>
                  <p className="text-zinc-500 animate-pulse">[BUSY] Simulating Load Testing...</p>
                </div>
              </div>
              <div className="bg-zinc-900/30 border border-zinc-800 rounded-2xl p-6 space-y-6">
                <h4 className="text-sm font-bold uppercase text-zinc-600 tracking-widest">Environment Checks</h4>
                {['Docker Stack', 'Database Migrations', 'A2A Sync State', 'Asset Bundler'].map(check => (
                  <div key={check} className="flex items-center justify-between">
                    <span className="text-sm text-zinc-300">{check}</span>
                    <ShieldCheck size={16} className="text-emerald-500" />
                  </div>
                ))}
                <button className="w-full mt-4 bg-sky-600 hover:bg-sky-500 text-white py-3 rounded-xl font-bold flex items-center justify-center gap-2 transition-all">
                  Run Full Suite <Zap size={16} />
                </button>
              </div>
            </div>
          </div>
        );

      case SDLCStage.PRODUCTION:
        return (
          <div className="flex flex-col items-center justify-center h-full p-6 animate-in slide-in-from-top-4 duration-700">
            <div className="max-w-2xl w-full bg-zinc-900/40 border border-zinc-800 rounded-[2rem] p-12 text-center relative overflow-hidden">
               <div className="absolute top-0 inset-x-0 h-1 bg-gradient-to-r from-transparent via-rose-500 to-transparent opacity-50" />
               <div className="w-24 h-24 rounded-3xl bg-rose-500/10 flex items-center justify-center mx-auto mb-8 animate-bounce">
                  <LayoutDashboard className="text-rose-500 w-12 h-12" />
               </div>
               <h2 className="text-4xl font-black text-white mb-4">Production Gateway</h2>
               <p className="text-zinc-500 mb-12 text-lg">Promote your stable build from <span className="text-sky-400 font-bold">PROJECT_ROOT/test</span> to the global infrastructure.</p>
               
               {deployStatus === 'idle' && (
                 <button 
                  onClick={handleDeploy}
                  className="bg-rose-600 hover:bg-rose-500 text-white px-12 py-5 rounded-2xl font-black text-xl flex items-center gap-4 mx-auto transition-all shadow-2xl shadow-rose-900/40 active:scale-95"
                 >
                   DEPLOY TO PRODUCTION <Rocket size={24} />
                 </button>
               )}

               {deployStatus === 'deploying' && (
                 <div className="space-y-6">
                    <div className="w-full bg-zinc-800 h-2 rounded-full overflow-hidden">
                      <div className="bg-rose-500 h-full animate-[progress_3s_ease-in-out_forwards]" style={{width: '0%'}} />
                    </div>
                    <p className="text-rose-400 font-mono animate-pulse">SYNCHRONIZING INFRASTRUCTURE NODES...</p>
                 </div>
               )}

               {deployStatus === 'live' && (
                 <div className="space-y-4 animate-in zoom-in duration-500">
                    <div className="bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 p-4 rounded-2xl flex items-center justify-center gap-3">
                      <ShieldCheck />
                      <span className="font-bold">SYSTEMS ARE LIVE - V4.2.0-STABLE</span>
                    </div>
                    <button onClick={() => setDeployStatus('idle')} className="text-zinc-600 hover:text-zinc-400 text-sm font-bold uppercase tracking-widest">
                      Prepare New Release
                    </button>
                 </div>
               )}
            </div>
          </div>
        );

      case SDLCStage.DOCTOR:
        return (
          <div className="flex flex-col h-full gap-4 p-4 animate-in fade-in duration-500">
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 h-full">
              <div className="flex flex-col gap-4">
                <div className="flex justify-between items-center">
                  <h3 className="text-xl font-bold flex items-center gap-2">
                    <Stethoscope className="text-red-500" /> Diagnosis Lab
                  </h3>
                  <button 
                    onClick={handleScan}
                    disabled={isScanning || !doctorCode}
                    className="bg-red-600 hover:bg-red-500 text-white px-6 py-2 rounded-lg font-bold disabled:opacity-50 transition-all shadow-lg shadow-red-900/20"
                  >
                    {isScanning ? 'Scanning...' : 'Run Diagnostics'}
                  </button>
                </div>
                <textarea
                  value={doctorCode}
                  onChange={(e) => setDoctorCode(e.target.value)}
                  placeholder="Paste your source code here for analysis... (Simulates scanning project root)"
                  className="flex-1 bg-black border border-zinc-800 rounded-xl p-4 font-mono text-sm text-zinc-300 focus:outline-none focus:border-red-500/50 resize-none shadow-inner"
                />
              </div>
              <div className="flex flex-col gap-4 overflow-y-auto">
                <h3 className="text-xl font-bold text-zinc-400">Diagnostic Report</h3>
                {diagReport ? (
                  <div className="space-y-4">
                    <div className={`p-4 rounded-xl border ${
                      diagReport.severity === 'critical' ? 'bg-red-950/20 border-red-900' : 'bg-zinc-900 border-zinc-800'
                    }`}>
                      <div className="flex justify-between mb-2">
                        <span className="text-xs font-bold uppercase tracking-widest text-zinc-500">Summary</span>
                        <span className={`text-xs font-bold uppercase px-2 py-0.5 rounded ${
                          diagReport.severity === 'critical' ? 'bg-red-600 text-white' : 'bg-zinc-800 text-zinc-400'
                        }`}>
                          {diagReport.severity}
                        </span>
                      </div>
                      <p className="text-zinc-200">{diagReport.summary}</p>
                    </div>

                    <div className="bg-zinc-900 border border-zinc-800 p-4 rounded-xl">
                      <h4 className="text-sm font-bold text-zinc-500 mb-3 uppercase">Findings</h4>
                      <ul className="space-y-2">
                        {diagReport.findings.map((f: string, i: number) => (
                          <li key={i} className="flex gap-3 text-sm text-zinc-300">
                            <span className="text-red-500">•</span> {f}
                          </li>
                        ))}
                      </ul>
                    </div>

                    <div className="bg-black border border-zinc-800 p-4 rounded-xl relative">
                      <div className="flex justify-between items-center mb-3">
                        <h4 className="text-sm font-bold text-emerald-500 uppercase flex items-center gap-2">
                          <ClipboardCheck size={14} /> Suggested Fix (CLI Ready)
                        </h4>
                        <div className="flex gap-2">
                           <button 
                            onClick={() => navigator.clipboard.writeText(diagReport.suggestedFix)}
                            className="text-[10px] text-zinc-500 hover:text-zinc-300 border border-zinc-800 px-2 py-1 rounded"
                          >
                            Copy Fix
                          </button>
                          <button 
                             onClick={() => setCurrentStage(SDLCStage.DEVELOPMENT)}
                             className="text-[10px] bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 px-2 py-1 rounded hover:bg-emerald-500/20"
                          >
                            Send to CLI
                          </button>
                        </div>
                      </div>
                      <pre className="text-xs text-zinc-400 font-mono whitespace-pre-wrap leading-relaxed max-h-48 overflow-y-auto">
                        {diagReport.suggestedFix}
                      </pre>
                    </div>
                  </div>
                ) : (
                  <div className="flex-1 border-2 border-dashed border-zinc-800 rounded-xl flex items-center justify-center text-zinc-700 italic">
                    Ready to scan code for errors.
                  </div>
                )}
              </div>
            </div>
          </div>
        );

      case SDLCStage.ARCHIVE:
        return (
          <div className="p-6 animate-in fade-in duration-500">
             <div className="flex items-center gap-3 mb-8">
                <Archive className="text-zinc-600" />
                <h2 className="text-2xl font-bold text-zinc-400 tracking-tight underline decoration-zinc-800 decoration-4 underline-offset-8">Project Archive Vault</h2>
             </div>
             <div className="space-y-3">
                {[1, 2, 3, 4].map(i => (
                  <div key={i} className="bg-zinc-900/30 border border-zinc-800/50 p-4 rounded-xl flex items-center justify-between hover:bg-zinc-800/30 transition-colors group">
                    <div className="flex items-center gap-4">
                      <div className="w-10 h-10 rounded-lg bg-zinc-800 flex items-center justify-center text-zinc-500">
                        <Archive size={18} />
                      </div>
                      <div>
                        <h4 className="text-zinc-300 font-bold">Legacy System Beta-0{i}</h4>
                        <p className="text-xs text-zinc-600">Archived on {new Date().toLocaleDateString()} • Size: 45.2MB</p>
                      </div>
                    </div>
                    <button className="text-zinc-500 hover:text-sky-500 p-2 opacity-0 group-hover:opacity-100 transition-opacity">
                      <ChevronRight size={20} />
                    </button>
                  </div>
                ))}
             </div>
          </div>
        );

      default:
        return (
          <div className="flex items-center justify-center h-full text-zinc-600 italic">
            Stage {currentStage} implementation in progress...
          </div>
        );
    }
  };

  return (
    <div className="flex h-screen w-screen bg-[#0a0a0b] text-zinc-100 overflow-hidden font-sans selection:bg-emerald-500/30">
      <style>{`
        @keyframes progress {
          0% { width: 0%; }
          100% { width: 100%; }
        }
      `}</style>

      {/* Sidebar */}
      <aside className={`${sidebarOpen ? 'w-64' : 'w-20'} flex flex-col border-r border-zinc-800 bg-[#09090b] transition-all duration-300 z-50`}>
        <div className="p-6 flex items-center justify-between">
          {sidebarOpen ? (
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500 to-sky-600 flex items-center justify-center shadow-lg shadow-emerald-500/20">
                <Brain className="text-white w-6 h-6" />
              </div>
              <span className="font-black text-2xl tracking-tighter">NEXUS</span>
            </div>
          ) : (
            <div className="w-10 h-10 rounded-xl bg-emerald-500/10 flex items-center justify-center mx-auto cursor-pointer hover:bg-emerald-500/20 transition-colors">
               <Brain className="text-emerald-500 w-6 h-6" />
            </div>
          )}
        </div>

        <nav className="flex-1 px-3 space-y-1 mt-4">
          {navItems.map((item) => (
            <button
              key={item.stage}
              onClick={() => setCurrentStage(item.stage)}
              className={`w-full flex items-center gap-4 px-4 py-3 rounded-xl transition-all group relative ${
                currentStage === item.stage 
                  ? 'bg-zinc-800/50 text-white border border-zinc-700/50 shadow-lg' 
                  : 'text-zinc-500 hover:bg-zinc-900/50 hover:text-zinc-300'
              }`}
            >
              <item.icon size={20} className={`${currentStage === item.stage ? item.color : 'text-zinc-500 group-hover:text-zinc-400'}`} />
              {sidebarOpen && <span className="font-bold tracking-tight">{item.label}</span>}
              {currentStage === item.stage && (
                <div className={`absolute left-0 w-1 h-6 rounded-r-full ${item.color.replace('text-', 'bg-')}`} />
              )}
            </button>
          ))}
        </nav>

        <div className="p-4 border-t border-zinc-800 bg-black/20 space-y-2">
          <button 
            onClick={() => setBrainPanelOpen(true)}
            className={`w-full flex items-center gap-3 p-3 rounded-xl bg-emerald-500/5 hover:bg-emerald-500/10 border border-emerald-500/10 transition-all ${!sidebarOpen && 'justify-center'}`}
          >
             <Brain size={20} className="text-emerald-500" />
             {sidebarOpen && <div className="flex flex-col items-start">
               <span className="text-[10px] font-black text-emerald-500 uppercase">Synced Session</span>
               <span className="text-xs text-zinc-400 font-mono truncate max-w-[120px]">{session?.sessionId || 'Connecting...'}</span>
             </div>}
          </button>
          
          <div className="flex gap-2">
            <button className="flex-1 p-2 text-zinc-600 hover:text-zinc-300 transition-colors bg-zinc-900/50 rounded-lg border border-zinc-800"><Bell size={18} /></button>
            <button className="flex-1 p-2 text-zinc-600 hover:text-zinc-300 transition-colors bg-zinc-900/50 rounded-lg border border-zinc-800"><Settings size={18} /></button>
            <button 
              onClick={() => setSidebarOpen(!sidebarOpen)} 
              className="p-2 text-zinc-500 hover:text-zinc-300 transition-colors ml-auto"
            >
              {sidebarOpen ? <X size={18} /> : <Menu size={18} />}
            </button>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col overflow-hidden relative">
        {/* Header */}
        <header className="h-16 border-b border-zinc-800 bg-[#09090b]/60 backdrop-blur-xl flex items-center justify-between px-8 z-40">
          <div className="flex items-center gap-4">
            <h1 className="text-sm font-black flex items-center gap-3">
              <span className="bg-zinc-800 text-zinc-500 px-2 py-0.5 rounded text-[10px] uppercase font-mono tracking-widest">Stage</span>
              {navItems.find(n => n.stage === currentStage)?.label.toUpperCase()}
              <span className="text-zinc-700 font-mono font-normal">/ ROOT / {currentStage.toLowerCase()}</span>
            </h1>
          </div>

          <div className="flex items-center gap-8">
            <div className="hidden lg:flex items-center gap-10 text-[10px] font-mono uppercase tracking-[0.2em] text-zinc-600">
              <div className="flex flex-col items-end">
                <span className="text-zinc-800">Scanned LOC</span>
                <span className="text-zinc-400 font-bold">{session?.stats.linesScanned.toLocaleString() || 0}</span>
              </div>
              <div className="flex flex-col items-end">
                <span className="text-zinc-800">Fixed Bugs</span>
                <span className="text-emerald-500 font-bold">{session?.stats.bugsFixed || 0}</span>
              </div>
              <div className="flex flex-col items-end">
                <span className="text-zinc-800">A2A Status</span>
                <span className="text-sky-500 font-bold">TUNNEL SECURE</span>
              </div>
            </div>
            <div className="flex items-center gap-3">
               <div className="text-right hidden sm:block">
                  <p className="text-xs font-bold text-zinc-300">Nexus Operator</p>
                  <p className="text-[10px] text-zinc-600 uppercase tracking-tighter">Level 0 Admin</p>
               </div>
               <div className="w-10 h-10 rounded-xl bg-zinc-800 border border-zinc-700 flex items-center justify-center font-black text-zinc-500">
                NO
              </div>
            </div>
          </div>
        </header>

        {/* Viewport */}
        <div className="flex-1 relative overflow-hidden bg-[radial-gradient(circle_at_50%_50%,#0e0e11,0,#0a0a0b_100%)]">
          <div className="absolute inset-0 p-6">
            {renderContent()}
          </div>
        </div>

        {/* Footer Stats Bar */}
        <footer className="h-8 border-t border-zinc-800 bg-[#09090b] flex items-center justify-between px-4 text-[9px] font-mono text-zinc-700 font-bold tracking-widest">
          <div className="flex items-center gap-6">
            <span className="flex items-center gap-2 uppercase"><div className="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_#10b981]" /> Brain Link Alpha: ON</span>
            <span className="flex items-center gap-2 uppercase"><div className="w-2 h-2 rounded-full bg-sky-500 shadow-[0_0_8px_#0ea5e9]" /> Terminals: SYNCED</span>
          </div>
          <div className="flex items-center gap-6 uppercase">
            <span>MEM: 12.8GB</span>
            <span>GPU: 42%</span>
            <span>Uptime: 42:11:09</span>
          </div>
        </footer>

        {/* Brain Slide Panel */}
        {brainPanelOpen && (
          <div className="absolute inset-0 z-50 flex justify-end">
            <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setBrainPanelOpen(false)} />
            <div className="w-96 bg-[#0c0c0e] border-l border-zinc-800 h-full relative z-10 shadow-2xl animate-in slide-in-from-right duration-300 flex flex-col">
              <div className="p-6 border-b border-zinc-800 flex justify-between items-center bg-black/20">
                 <div className="flex items-center gap-3">
                    <Brain className="text-emerald-500" />
                    <h2 className="text-lg font-black tracking-tight uppercase">Brain Persistence</h2>
                 </div>
                 <button onClick={() => setBrainPanelOpen(false)} className="text-zinc-600 hover:text-white p-2 transition-colors"><X /></button>
              </div>
              
              <div className="flex-1 overflow-y-auto p-6 space-y-8 custom-scrollbar">
                {/* Statistics Card */}
                <div className="bg-emerald-500/5 border border-emerald-500/10 p-4 rounded-2xl">
                   <h4 className="text-xs font-bold text-emerald-500 mb-3 uppercase tracking-tighter">Live Statistics</h4>
                   <div className="grid grid-cols-2 gap-4">
                      <div>
                        <p className="text-[10px] text-zinc-600 uppercase font-black">Prompts</p>
                        <p className="text-xl font-black text-zinc-300">{session?.stats.promptsSent}</p>
                      </div>
                      <div>
                        <p className="text-[10px] text-zinc-600 uppercase font-black">Accuracy</p>
                        <p className="text-xl font-black text-zinc-300">99.1%</p>
                      </div>
                   </div>
                </div>

                {/* Runbacks Section */}
                <div>
                   <h4 className="text-[10px] uppercase tracking-widest text-zinc-600 font-black mb-4 flex items-center gap-2">
                     <History size={12} /> Recent Runbacks
                   </h4>
                   <div className="space-y-3">
                      {session?.runbacks.map(rb => (
                        <div key={rb.id} className="bg-zinc-900 border border-zinc-800 p-3 rounded-lg flex flex-col gap-1 hover:border-sky-500/30 transition-colors">
                          <code className="text-xs text-sky-400">$ {rb.command}</code>
                          <span className="text-[9px] text-zinc-700 uppercase font-bold">{new Date(rb.timestamp).toLocaleString()}</span>
                        </div>
                      ))}
                   </div>
                </div>

                {/* Snippets Section */}
                <div>
                   <div className="flex justify-between items-center mb-4">
                      <h4 className="text-[10px] uppercase tracking-widest text-zinc-600 font-black flex items-center gap-2">
                        <ClipboardCheck size={12} /> Sync Snippets
                      </h4>
                      <button 
                        onClick={() => setIsAddingSnippet(!isAddingSnippet)}
                        className={`text-[10px] px-2 py-1 rounded-md font-bold uppercase transition-all flex items-center gap-1 ${
                          isAddingSnippet 
                            ? 'bg-red-500/20 text-red-400 border border-red-500/30' 
                            : 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30'
                        }`}
                      >
                        {isAddingSnippet ? <X size={10} /> : <Plus size={10} />}
                        {isAddingSnippet ? 'Cancel' : 'New'}
                      </button>
                   </div>

                   {/* Add Snippet Form */}
                   {isAddingSnippet && (
                     <div className="bg-zinc-900 border border-emerald-500/30 p-4 rounded-xl mb-6 space-y-4 animate-in slide-in-from-top-4 duration-300">
                        <div className="space-y-2">
                          <label className="text-[10px] text-zinc-500 uppercase font-bold">Title</label>
                          <input 
                            value={newSnipTitle}
                            onChange={(e) => setNewSnipTitle(e.target.value)}
                            placeholder="Snippet Title..."
                            className="w-full bg-black border border-zinc-800 rounded px-3 py-1.5 text-xs text-zinc-200 focus:outline-none focus:border-emerald-500/50"
                          />
                        </div>
                        <div className="space-y-2">
                          <label className="text-[10px] text-zinc-500 uppercase font-bold">Language</label>
                          <select 
                            value={newSnipLang}
                            onChange={(e) => setNewSnipLang(e.target.value)}
                            className="w-full bg-black border border-zinc-800 rounded px-3 py-1.5 text-xs text-zinc-400 focus:outline-none focus:border-emerald-500/50"
                          >
                            <option value="typescript">TypeScript</option>
                            <option value="javascript">JavaScript</option>
                            <option value="python">Python</option>
                            <option value="bash">Bash</option>
                            <option value="json">JSON</option>
                          </select>
                        </div>
                        <div className="space-y-2">
                          <label className="text-[10px] text-zinc-500 uppercase font-bold">Code</label>
                          <textarea 
                            value={newSnipCode}
                            onChange={(e) => setNewSnipCode(e.target.value)}
                            placeholder="Paste or write code..."
                            rows={4}
                            className="w-full bg-black border border-zinc-800 rounded px-3 py-1.5 text-[10px] font-mono text-emerald-100/70 focus:outline-none focus:border-emerald-500/50 resize-none"
                          />
                        </div>
                        <button 
                          onClick={handleAddSnippet}
                          disabled={!newSnipTitle || !newSnipCode}
                          className="w-full bg-emerald-600 hover:bg-emerald-500 text-white text-[10px] font-bold py-2 rounded transition-all disabled:opacity-50 flex items-center justify-center gap-2"
                        >
                          <Save size={12} /> SAVE TO BRAIN
                        </button>
                     </div>
                   )}

                   <div className="space-y-4">
                      {session?.codeSnippets.map(snip => (
                        <div key={snip.id} className="bg-black border border-zinc-800 p-4 rounded-xl group relative hover:border-zinc-700 transition-colors">
                          <div className="flex justify-between items-start mb-2">
                             <h5 className="text-xs font-bold text-zinc-400">{snip.title}</h5>
                             <div className="flex gap-2">
                               <button 
                                onClick={() => navigator.clipboard.writeText(snip.code)}
                                className="text-[9px] opacity-0 group-hover:opacity-100 bg-zinc-800 px-2 py-0.5 rounded text-zinc-500 transition-opacity hover:text-zinc-300"
                               >
                                Copy
                               </button>
                               <button 
                                onClick={() => handleDeleteSnippet(snip.id)}
                                className="text-[9px] opacity-0 group-hover:opacity-100 bg-red-500/10 px-2 py-0.5 rounded text-red-500/50 transition-opacity hover:text-red-400 hover:bg-red-500/20"
                               >
                                <Trash2 size={10} />
                               </button>
                             </div>
                          </div>
                          <div className="flex items-center gap-2 mb-2">
                             <span className="text-[9px] px-1.5 py-0.5 bg-zinc-900 rounded text-zinc-600 font-mono uppercase">{snip.language}</span>
                          </div>
                          <pre className="text-[10px] text-zinc-600 overflow-x-auto max-h-32 leading-tight custom-scrollbar">{snip.code}</pre>
                        </div>
                      ))}
                      {session?.codeSnippets.length === 0 && (
                        <div className="text-center py-8 border-2 border-dashed border-zinc-900 rounded-2xl text-zinc-800 italic text-xs">
                          No persistent snippets found.
                        </div>
                      )}
                   </div>
                </div>
              </div>

              <div className="p-6 border-t border-zinc-800 bg-black/40">
                 <button className="w-full bg-emerald-600/10 hover:bg-emerald-600/20 text-emerald-500 py-3 rounded-xl font-bold uppercase tracking-widest text-xs transition-all border border-emerald-500/20 flex items-center justify-center gap-2 group">
                   <Zap size={14} className="group-hover:scale-125 transition-transform" />
                   Force A2A Sync Now
                 </button>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
};

export default App;
