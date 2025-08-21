import { Avatar } from '@heroui/react';
import { memo, useMemo } from 'react';

import DefaultIcon from './DefaultIcon';
import { providerMappings } from './providerMappings';
import { ProviderIconProps } from './types';

const ProviderIconComponent = memo<ProviderIconProps>(
  ({ provider: originProvider, size = 24, type = 'color', className, style }) => {
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
      className,
      style,
      ...mapping?.props,
    };

    if (type === 'avatar') {
      return mapping?.Icon ? (
        <div className="flex items-center justify-center rounded-md overflow-hidden">
          <mapping.Icon {...iconProps} />
        </div>
      ) : (
        <Avatar
          size={size <= 20 ? 'sm' : size <= 28 ? 'md' : 'lg'}
          className={className}
          name={originProvider?.charAt(0).toUpperCase() || '?'}
          style={style}
        />
      );
    }

    return mapping?.Icon ? (
      <mapping.Icon {...iconProps} />
    ) : (
      <DefaultIcon size={size} className={className} style={style} />
    );
  },
);

ProviderIconComponent.displayName = 'ProviderIcon';

export { default as ProviderCombine } from './ProviderCombine';
export { type ProviderIconProps, type ProviderMapping } from './types';
export default ProviderIconComponent;