import type { Step } from 'react-joyride';

export const newAppTourSteps: Step[] = [
  {
    target: 'body',
    placement: 'center',
    title: 'tour.newApp.welcome.title',
    content: 'tour.newApp.welcome.content',
    disableBeacon: true,
  },
  {
    target: '[data-tour="repo-list"]',
    title: 'tour.newApp.repo.title',
    content: 'tour.newApp.repo.content',
  },
  {
    target: '[data-tour="deploy-config"]',
    title: 'tour.newApp.config.title',
    content: 'tour.newApp.config.content',
  },
];
