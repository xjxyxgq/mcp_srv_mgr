import React from 'react';

export interface IconType {
  size?: number;
  className?: string;
  style?: React.CSSProperties;
}

export interface ProviderIconProps extends IconType {
  provider?: string;
  type?: 'mono' | 'color' | 'avatar';
}

export interface ProviderMapping {
  Icon: React.ComponentType<IconType>;
  keywords: string[];
  combineMultiple?: number;
  props?: Record<string, unknown>;
}