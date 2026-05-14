import React, { createContext, useContext, useEffect, useRef } from "react";
import { subscribeToJobEvents } from "../src/api/client";
import type { JobEvent } from "../src/api/types";

type JobEventListener = (event: JobEvent) => void;

interface JobEventContextType {
  subscribe: (listener: JobEventListener) => () => void;
}

const JobEventContext = createContext<JobEventContextType | undefined>(undefined);

export const JobEventProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const listenersRef = useRef<Set<JobEventListener>>(new Set());

  useEffect(() => {
    // Global subscription to all job events
    const unsubscribe = subscribeToJobEvents((event) => {
      // Direct notification of all listeners bypasses React state batching
      listenersRef.current.forEach((listener) => listener(event));
    });

    return () => {
      unsubscribe();
    };
  }, []);

  const subscribe = (listener: JobEventListener) => {
    listenersRef.current.add(listener);
    return () => {
      listenersRef.current.delete(listener);
    };
  };

  return (
    <JobEventContext.Provider value={{ subscribe }}>
      {children}
    </JobEventContext.Provider>
  );
};

export const useJobEvents = () => {
  const context = useContext(JobEventContext);
  if (context === undefined) {
    throw new Error("useJobEvents must be used within a JobEventProvider");
  }
  return context;
};

