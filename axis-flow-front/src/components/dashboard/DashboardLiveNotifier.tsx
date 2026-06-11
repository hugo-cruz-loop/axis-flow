import React from 'react';

interface LiveNotifierProps {
  isRefetching: boolean;
  isError: boolean;
  widgetName: string;
}

export const DashboardLiveNotifier: React.FC<LiveNotifierProps> = ({
  isRefetching,
  isError,
  widgetName,
}) => {
  const announcement = isRefetching
    ? `Refreshing data for ${widgetName}...`
    : isError
      ? `Failed to update ${widgetName} data.`
      : `${widgetName} metrics updated successfully.`;

  return (
    <div 
      className="sr-only" 
      role="status" 
      aria-live="polite" 
      aria-atomic="true"
      data-testid="live-notifier"
    >
      {announcement}
    </div>
  );
};
