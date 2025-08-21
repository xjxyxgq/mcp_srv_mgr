import { Avatar } from '@heroui/react';
import React from 'react';

import LocalProviderIcon from './ProviderIcon/index';

interface ProviderIconProps {
  providerId: string;
  name: string;
  size?: number;
  className?: string;
  fallbackUrl?: string;
}

const ProviderIcon: React.FC<ProviderIconProps> = ({ 
  providerId, 
  name, 
  size = 24,
  className = '',
  fallbackUrl 
}) => {
  try {
    // Use our local provider icon implementation
    return (
      <div className={`inline-flex items-center justify-center ${className}`} style={{ borderRadius: 6 }}>
        <LocalProviderIcon
          provider={providerId}
          size={size}
          type="avatar"
        />
      </div>
    );
  } catch (error) {
    console.warn(`Icon not found for provider: ${providerId}`, error);
    
    // Fallback to Avatar with either fallbackUrl or first letter
    return (
      <Avatar
        size={size <= 20 ? 'sm' : size <= 28 ? 'md' : 'lg'}
        src={fallbackUrl}
        className={className}
        name={name}
        fallback={name.charAt(0).toUpperCase()}
      />
    );
  }
};

export default ProviderIcon;