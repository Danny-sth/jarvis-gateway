/**
 * JARVIS Calendar Reminder
 * Google Apps Script for sending calendar reminders via webhook
 *
 * Setup:
 * 1. Go to script.google.com
 * 2. Create new project
 * 3. Paste this code
 * 4. Set WEBHOOK_URL and WEBHOOK_TOKEN in Script Properties
 * 5. Run setupTrigger() once
 */

// Configuration - Set these in Script Properties (File -> Project Properties -> Script Properties)
function getConfig() {
  const props = PropertiesService.getScriptProperties();
  return {
    webhookUrl: props.getProperty('WEBHOOK_URL') || 'https://on-za-menya.online/api/calendar',
    webhookToken: props.getProperty('WEBHOOK_TOKEN') || '',
    reminderMinutes: parseInt(props.getProperty('REMINDER_MINUTES') || '15'),
    calendarId: props.getProperty('CALENDAR_ID') || 'primary'
  };
}

/**
 * Main function - runs every 5 minutes via trigger
 */
function checkUpcomingEvents() {
  const config = getConfig();

  if (!config.webhookToken) {
    console.error('WEBHOOK_TOKEN not set in Script Properties');
    return;
  }

  const now = new Date();
  const checkStart = new Date(now.getTime() + (config.reminderMinutes - 2) * 60 * 1000);
  const checkEnd = new Date(now.getTime() + (config.reminderMinutes + 3) * 60 * 1000);

  const calendar = CalendarApp.getCalendarById(config.calendarId);
  if (!calendar) {
    console.error('Calendar not found: ' + config.calendarId);
    return;
  }

  const events = calendar.getEvents(checkStart, checkEnd);
  const processedKey = 'processed_events';
  const processed = getProcessedEvents();

  for (const event of events) {
    const eventId = event.getId();
    const eventStart = event.getStartTime();

    // Skip if already processed
    if (processed[eventId]) {
      continue;
    }

    // Skip all-day events
    if (event.isAllDayEvent()) {
      continue;
    }

    // Send reminder
    const success = sendReminder(config, event);

    if (success) {
      // Mark as processed
      processed[eventId] = eventStart.getTime();
      console.log('Reminder sent for: ' + event.getTitle());
    }
  }

  // Save processed events and cleanup old ones
  cleanupAndSaveProcessed(processed);
}

/**
 * Send reminder webhook
 */
function sendReminder(config, event) {
  const payload = {
    type: 'reminder',
    event: {
      title: event.getTitle(),
      start_time: event.getStartTime().toISOString(),
      end_time: event.getEndTime().toISOString(),
      location: event.getLocation() || '',
      meet_link: extractMeetLink(event),
      event_id: event.getId()
    },
    minutes_before: config.reminderMinutes
  };

  const options = {
    method: 'post',
    contentType: 'application/json',
    headers: {
      'Authorization': 'Bearer ' + config.webhookToken
    },
    payload: JSON.stringify(payload),
    muteHttpExceptions: true
  };

  try {
    const response = UrlFetchApp.fetch(config.webhookUrl, options);
    const code = response.getResponseCode();

    if (code === 200) {
      return true;
    } else {
      console.error('Webhook failed: ' + code + ' - ' + response.getContentText());
      return false;
    }
  } catch (e) {
    console.error('Webhook error: ' + e.message);
    return false;
  }
}

/**
 * Extract Google Meet link from event
 */
function extractMeetLink(event) {
  // Try to get from hangout link
  const hangoutLink = event.getHangoutLink && event.getHangoutLink();
  if (hangoutLink) {
    return hangoutLink;
  }

  // Try to find in description
  const description = event.getDescription() || '';
  const meetMatch = description.match(/https:\/\/meet\.google\.com\/[a-z-]+/i);
  if (meetMatch) {
    return meetMatch[0];
  }

  // Try to find zoom link
  const zoomMatch = description.match(/https:\/\/[a-z0-9]+\.zoom\.us\/j\/\d+/i);
  if (zoomMatch) {
    return zoomMatch[0];
  }

  return '';
}

/**
 * Get processed events from cache
 */
function getProcessedEvents() {
  const cache = CacheService.getScriptCache();
  const data = cache.get('processed_events');

  if (data) {
    try {
      return JSON.parse(data);
    } catch (e) {
      return {};
    }
  }

  return {};
}

/**
 * Cleanup old events and save
 */
function cleanupAndSaveProcessed(processed) {
  const now = new Date().getTime();
  const maxAge = 24 * 60 * 60 * 1000; // 24 hours

  // Remove old entries
  for (const eventId in processed) {
    if (now - processed[eventId] > maxAge) {
      delete processed[eventId];
    }
  }

  // Save to cache (6 hours max)
  const cache = CacheService.getScriptCache();
  cache.put('processed_events', JSON.stringify(processed), 21600);
}

/**
 * Setup trigger - run once manually
 */
function setupTrigger() {
  // Remove existing triggers
  const triggers = ScriptApp.getProjectTriggers();
  for (const trigger of triggers) {
    if (trigger.getHandlerFunction() === 'checkUpcomingEvents') {
      ScriptApp.deleteTrigger(trigger);
    }
  }

  // Create new trigger - every 5 minutes
  ScriptApp.newTrigger('checkUpcomingEvents')
    .timeBased()
    .everyMinutes(5)
    .create();

  console.log('Trigger created: checkUpcomingEvents every 5 minutes');
}

/**
 * Remove all triggers
 */
function removeTriggers() {
  const triggers = ScriptApp.getProjectTriggers();
  for (const trigger of triggers) {
    ScriptApp.deleteTrigger(trigger);
  }
  console.log('All triggers removed');
}

/**
 * Test function - send test reminder
 */
function testReminder() {
  const config = getConfig();

  const testEvent = {
    getTitle: () => 'Test Event',
    getStartTime: () => new Date(Date.now() + 15 * 60 * 1000),
    getEndTime: () => new Date(Date.now() + 75 * 60 * 1000),
    getLocation: () => 'Test Location',
    getDescription: () => '',
    getId: () => 'test-' + Date.now(),
    getHangoutLink: () => ''
  };

  const success = sendReminder(config, testEvent);
  console.log('Test reminder ' + (success ? 'sent successfully' : 'failed'));
}
