import type { Step } from 'react-joyride';

export const dashboardTourSteps: Step[] = [
  {
    target: 'body',
    placement: 'center',
    title: 'tour.dashboard.welcome.title',
    content: 'tour.dashboard.welcome.content',
    disableBeacon: true,
  },
  {
    target: '[data-tour="stats-grid"]',
    title: 'tour.dashboard.stats.title',
    content: 'tour.dashboard.stats.content',
  },
  {
    target: '[data-tour="new-app-btn"]',
    title: 'tour.dashboard.newApp.title',
    content: 'tour.dashboard.newApp.content',
  },
  {
    target: '[data-tour="apps-list"]',
    title: 'tour.dashboard.apps.title',
    content: 'tour.dashboard.apps.content',
  },
  {
    target: '[data-tour="recent-deploys"]',
    title: 'tour.dashboard.deploys.title',
    content: 'tour.dashboard.deploys.content',
  },
];
