import React from 'react';

import { IconType } from './types';

const DefaultIcon: React.FC<IconType> = ({ size = 24, className, style }) => (
  <div
    className={`flex items-center justify-center rounded-md bg-gray-200 text-gray-600 ${className || ''}`}
    style={{
      width: size,
      height: size,
      fontSize: size * 0.6,
      fontWeight: 'bold',
      ...style,
    }}
  >
    ?
  </div>
);

export default DefaultIcon;