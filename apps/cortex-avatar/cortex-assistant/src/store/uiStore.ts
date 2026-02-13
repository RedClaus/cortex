import { create } from 'zustand';

interface UIState {
  commandPaletteOpen: boolean;
  sidebarOpen: boolean;
  settingsOpen: boolean;
  meetingInfoOpen: boolean;
  analysisOpen: boolean;
  exportModalOpen: boolean;
  
  openCommandPalette: () => void;
  closeCommandPalette: () => void;
  toggleCommandPalette: () => void;
  
  openSidebar: () => void;
  closeSidebar: () => void;
  toggleSidebar: () => void;
  
  openSettings: () => void;
  closeSettings: () => void;
  
  openMeetingInfo: () => void;
  closeMeetingInfo: () => void;
  
  openAnalysis: () => void;
  closeAnalysis: () => void;
  
  openExportModal: () => void;
  closeExportModal: () => void;
  
  closeAllModals: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  commandPaletteOpen: false,
  sidebarOpen: true,
  settingsOpen: false,
  meetingInfoOpen: false,
  analysisOpen: false,
  exportModalOpen: false,

  openCommandPalette: () => set({ commandPaletteOpen: true }),
  closeCommandPalette: () => set({ commandPaletteOpen: false }),
  toggleCommandPalette: () =>
    set((state) => ({ commandPaletteOpen: !state.commandPaletteOpen })),

  openSidebar: () => set({ sidebarOpen: true }),
  closeSidebar: () => set({ sidebarOpen: false }),
  toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),

  openSettings: () => set({ settingsOpen: true }),
  closeSettings: () => set({ settingsOpen: false }),

  openMeetingInfo: () => set({ meetingInfoOpen: true }),
  closeMeetingInfo: () => set({ meetingInfoOpen: false }),

  openAnalysis: () => set({ analysisOpen: true }),
  closeAnalysis: () => set({ analysisOpen: false }),

  openExportModal: () => set({ exportModalOpen: true }),
  closeExportModal: () => set({ exportModalOpen: false }),

  closeAllModals: () =>
    set({
      commandPaletteOpen: false,
      settingsOpen: false,
      meetingInfoOpen: false,
      analysisOpen: false,
      exportModalOpen: false,
    }),
}));
