
import React, { useState, useRef, useEffect } from 'react';
import { AICLI, TerminalMessage } from '../types';
import { getCLIResponse } from '../services/geminiService';
import { Send, TerminalSquare, Upload, FileCode } from 'lucide-react';

interface TerminalProps {
  type: AICLI;
  onMessageSent?: (msg: string) => void;
}

export const Terminal: React.FC<TerminalProps> = ({ type, onMessageSent }) => {
  const [messages, setMessages] = useState<TerminalMessage[]>([
    { role: 'ai', text: `${type} v2.5.0 initialized. Environment: PROJECT_ROOT/${type.split(' ')[0].toLowerCase()}. Ready...`, timestamp: Date.now(), cli: type }
  ]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [attachedFiles, setAttachedFiles] = useState<string[]>([]);
  const scrollRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, isLoading]);

  const handleSend = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    if (!input.trim() && attachedFiles.length === 0) return;
    if (isLoading) return;

    const fullPrompt = attachedFiles.length > 0 
      ? `[FILES: ${attachedFiles.join(', ')}]\n${input}`
      : input;

    const userMsg: TerminalMessage = {
      role: 'user',
      text: fullPrompt,
      timestamp: Date.now(),
      cli: type
    };

    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setAttachedFiles([]);
    setIsLoading(true);

    const response = await getCLIResponse(type, fullPrompt);
    
    const aiMsg: TerminalMessage = {
      role: 'ai',
      text: response,
      timestamp: Date.now(),
      cli: type
    };

    setMessages(prev => [...prev, aiMsg]);
    setIsLoading(false);
    if (onMessageSent) onMessageSent(fullPrompt);
  };

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setAttachedFiles(prev => [...prev, file.name]);
    }
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  return (
    <div className="flex flex-col h-full bg-black/40 border border-zinc-800 rounded-lg overflow-hidden font-mono text-sm shadow-2xl">
      <div className="flex items-center justify-between px-4 py-2 bg-zinc-900/80 border-b border-zinc-800 backdrop-blur-sm">
        <div className="flex items-center gap-2">
          <TerminalSquare className="w-4 h-4 text-emerald-500" />
          <span className="font-bold text-zinc-400 text-xs tracking-wider">{type.toUpperCase()}</span>
        </div>
        <div className="flex gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full bg-zinc-800 border border-zinc-700" />
          <div className="w-2.5 h-2.5 rounded-full bg-zinc-800 border border-zinc-700" />
          <div className="w-2.5 h-2.5 rounded-full bg-zinc-800 border border-zinc-700" />
        </div>
      </div>

      <div ref={scrollRef} className="flex-1 p-4 overflow-y-auto space-y-4">
        {messages.map((m, i) => (
          <div key={i} className={`flex flex-col ${m.role === 'user' ? 'items-end' : 'items-start'}`}>
            <div className={`max-w-[95%] px-3 py-2 rounded-md ${
              m.role === 'user' 
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' 
                : 'bg-zinc-900/40 text-zinc-300 border border-zinc-800/60'
            }`}>
              <pre className="whitespace-pre-wrap font-mono leading-relaxed text-[13px]">{m.text}</pre>
            </div>
            <span className="text-[9px] text-zinc-700 mt-1 uppercase tracking-widest font-bold">
              {new Date(m.timestamp).toLocaleTimeString()} • {m.role}
            </span>
          </div>
        ))}
        {isLoading && (
          <div className="flex items-center gap-2 text-emerald-500/70 italic text-xs">
            <span className="w-2 h-2 bg-emerald-500 rounded-full animate-ping" />
            <span>Processing stream...</span>
          </div>
        )}
      </div>

      <div className="p-3 bg-zinc-900/50 border-t border-zinc-800 space-y-2">
        {attachedFiles.length > 0 && (
          <div className="flex flex-wrap gap-2 mb-2">
            {attachedFiles.map((f, i) => (
              <div key={i} className="flex items-center gap-2 bg-zinc-800 border border-zinc-700 px-2 py-1 rounded text-[11px] text-zinc-400">
                <FileCode size={12} className="text-sky-500" />
                {f}
                <button onClick={() => setAttachedFiles(prev => prev.filter((_, idx) => idx !== i))} className="hover:text-red-400">×</button>
              </div>
            ))}
          </div>
        )}
        <form onSubmit={handleSend} className="flex gap-2">
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileUpload}
            className="hidden"
          />
          <button 
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="p-2 bg-zinc-800 hover:bg-zinc-700 text-zinc-400 rounded border border-zinc-700 transition-colors"
            disabled={isLoading}
            title="Upload File"
          >
            <Upload size={16} />
          </button>
          <input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Send instruction..."
            className="flex-1 bg-black/60 border border-zinc-800 rounded px-3 py-1.5 focus:outline-none focus:border-emerald-500/50 text-emerald-100 placeholder:text-zinc-700"
            disabled={isLoading}
          />
          <button 
            type="submit"
            className="px-3 bg-emerald-600/20 hover:bg-emerald-600 text-emerald-400 hover:text-white rounded border border-emerald-600/30 transition-all disabled:opacity-50"
            disabled={isLoading || (!input.trim() && attachedFiles.length === 0)}
          >
            <Send size={16} />
          </button>
        </form>
      </div>
    </div>
  );
};
