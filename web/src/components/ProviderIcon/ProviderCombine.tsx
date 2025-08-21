import React, { memo, useMemo } from 'react';

import DefaultIcon from './DefaultIcon';
import { providerMappings } from './providerMappings';

export interface ProviderCombineProps extends Omit<React.HTMLAttributes<HTMLDivElement>, 'children'> {
  provider?: string;
  size?: number;
  type?: 'mono' | 'color';
}

const ProviderCombine = memo<ProviderCombineProps>(
  ({ provider: originProvider, size = 12, type: _type = 'color', className, style, ...rest }) => {
    const mapping = useMemo(() => {
      if (!originProvider) return;
      const provider = originProvider.toLowerCase();

      for (const item of providerMappings) {
        if (item.keywords.some((keyword) => keyword.toLowerCase() === provider)) {
          return item;
        }
      }
    }, [originProvider]);

    const iconProps = {
      size: size * (mapping?.combineMultiple || 1),
      ...mapping?.props,
    };

    const icon = mapping?.Icon ? (
      <mapping.Icon {...iconProps} />
    ) : (
      <DefaultIcon size={size} />
    );

    return (
      <div
        className={`inline-flex items-center justify-center ${className || ''}`}
        style={{
          height: size * 1.5,
          width: 'fit-content',
          ...style,
        }}
        {...rest}
      >
        {icon}
      </div>
    );
  },
);

ProviderCombine.displayName = 'ProviderCombine';

export default ProviderCombine;