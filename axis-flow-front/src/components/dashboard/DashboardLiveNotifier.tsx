import React, { useEffect, useState } from 'react';

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
  const [announcement, setAnnouncement] = useState('');

  useEffect(() => {
    if (isRefetching) {
      setAnnouncement(`Refreshing data for ${widgetName}...`);
    } else if (isError) {
      setAnnouncement(`Failed to update ${widgetName} data.`);
    } else {
      setAnnouncement(`${widgetName} metrics updated successfully.`);
    }
  }, [isRefetching, isError, widgetName]);

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
