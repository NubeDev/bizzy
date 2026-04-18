// content_review_qa.js — guided content review flow.

function handle(params) {
  // --- Chat mode: conversational Q&A via WebSocket ---
  if (params._answers !== undefined) {
    return chatMode(params._answers);
  }

  // --- Form mode: REST API ---
  if (!params._submit) {
    return getFormDefinition();
  }
  return submitForm(params);
}

// ---- Chat mode: one question at a time ----

function chatMode(answers) {
  if (!answers.content) {
    return {
      type: "question",
      field: "content",
      label: "Paste the content you'd like reviewed",
      input: "textarea",
      required: true,
      min_length: 10,
      placeholder: "Paste your marketing copy, blog post, email, etc."
    };
  }

  if (!answers.content_type) {
    return {
      type: "question",
      field: "content_type",
      label: "What type of content is this?",
      input: "select",
      required: true,
      options: [
        { value: "blog", label: "Blog post" },
        { value: "email", label: "Email campaign" },
        { value: "landing_page", label: "Landing page" },
        { value: "social", label: "Social media post" },
        { value: "case_study", label: "Case study" },
        { value: "datasheet", label: "Product datasheet" },
        { value: "press_release", label: "Press release" },
        { value: "other", label: "Other" }
      ]
    };
  }

  if (!answers.tone) {
    return {
      type: "question",
      field: "tone",
      label: "What tone are you going for?",
      input: "select",
      required: false,
      default: "professional",
      options: [
        { value: "professional", label: "Professional" },
        { value: "technical", label: "Technical" },
        { value: "casual", label: "Casual / approachable" },
        { value: "executive", label: "Executive / formal" }
      ]
    };
  }

  if (!answers.focus_areas) {
    return {
      type: "question",
      field: "focus_areas",
      label: "What should the review focus on?",
      input: "multi_select",
      options: [
        { value: "accuracy", label: "Technical accuracy" },
        { value: "tone", label: "Tone & voice" },
        { value: "value_prop", label: "Value proposition clarity" },
        { value: "cta", label: "Call to action" },
        { value: "grammar", label: "Grammar & readability" },
        { value: "seo", label: "SEO optimization" }
      ]
    };
  }

  // All done — build the prompt.
  return submitForm(answers);
}

function getFormDefinition() {
  return {
    type: "qa",
    title: "Content Review",
    description: "Paste your content and set review preferences for targeted feedback.",
    fields: [
      {
        name: "content",
        label: "Content to review",
        type: "textarea",
        required: true,
        min_length: 10,
        placeholder: "Paste your marketing copy, blog post, email, etc."
      },
      {
        name: "content_type",
        label: "Content type",
        type: "select",
        required: true,
        options: [
          { value: "blog", label: "Blog post" },
          { value: "email", label: "Email campaign" },
          { value: "landing_page", label: "Landing page" },
          { value: "social", label: "Social media post" },
          { value: "case_study", label: "Case study" },
          { value: "datasheet", label: "Product datasheet" },
          { value: "press_release", label: "Press release" },
          { value: "other", label: "Other" }
        ]
      },
      {
        name: "tone",
        label: "Desired tone",
        type: "select",
        required: false,
        default: "professional",
        options: [
          { value: "professional", label: "Professional" },
          { value: "technical", label: "Technical" },
          { value: "casual", label: "Casual / approachable" },
          { value: "executive", label: "Executive / formal" }
        ]
      },
      {
        name: "focus_areas",
        label: "What should the review focus on?",
        type: "multi_select",
        required: false,
        options: [
          { value: "accuracy", label: "Technical accuracy" },
          { value: "tone", label: "Tone & voice" },
          { value: "value_prop", label: "Value proposition clarity" },
          { value: "cta", label: "Call to action" },
          { value: "grammar", label: "Grammar & readability" },
          { value: "seo", label: "SEO optimization" }
        ]
      }
    ]
  };
}

function submitForm(params) {
  var errors = [];

  if (!params.content || params.content.length < 10) {
    errors.push({ field: "content", message: "Please paste at least 10 characters of content" });
  }
  if (!params.content_type) {
    errors.push({ field: "content_type", message: "Please select a content type" });
  }

  if (errors.length > 0) {
    return { type: "validation_error", errors: errors };
  }

  // Build the prompt from the template.
  var template = files.read("prompts/content-review.md");
  var parts = template.split("---");
  var body = parts.slice(2).join("---").trim();

  body = body.replace(/\{\{content\}\}/g, params.content);

  // Append review context.
  body += "\n\nContent type: " + params.content_type;
  body += "\nDesired tone: " + (params.tone || "professional");

  if (params.focus_areas) {
    body += "\nPriority review areas: " + params.focus_areas;
  }

  return { type: "prompt", prompt: body };
}
