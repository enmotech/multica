import { createStore } from "zustand/vanilla";
import { useStore } from "zustand";

interface ConfigState {
  cdnDomain: string;
  allowSignup: boolean;
  googleClientId: string;
  allowedEmailDomains: string;
  setCdnDomain: (domain: string) => void;
  setAuthConfig: (config: { allowSignup: boolean; googleClientId?: string; allowedEmailDomains?: string }) => void;
}

export const configStore = createStore<ConfigState>((set) => ({
  cdnDomain: "",
  allowSignup: true,
  googleClientId: "",
  allowedEmailDomains: "",
  setCdnDomain: (domain) => set({ cdnDomain: domain }),
  setAuthConfig: ({ allowSignup, googleClientId = "", allowedEmailDomains = "" }) =>
    set({ allowSignup, googleClientId, allowedEmailDomains }),
}));

export function useConfigStore(): ConfigState;
export function useConfigStore<T>(selector: (state: ConfigState) => T): T;
export function useConfigStore<T>(selector?: (state: ConfigState) => T) {
  return useStore(configStore, selector as (state: ConfigState) => T);
}
