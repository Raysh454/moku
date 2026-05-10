
import React from 'react';

export const LoadingSpinner: React.FC = () => (
  <div className="flex items-center justify-center p-8">
    <div className="animate-spin rounded-full h-10 w-10 border-4 border-accent/20 border-t-accent shadow-[0_0_15px_rgba(81,112,255,0.2)]"></div>
  </div>
);
